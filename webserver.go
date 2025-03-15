package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/a-h/templ"
	"golang.org/x/sync/errgroup"
)

const (
	defaultHTTPPort                 = 8081
	httpShutdownPreStopDelaySeconds = 0
	httpShutdownTimeoutSeconds      = 0
	defaultBackgroundTimeoutSeconds = 2
)

//go:embed assets
var fsAssets embed.FS

type responseRecorder struct {
	w      http.ResponseWriter
	status int
}

func (rr *responseRecorder) WriteHeader(status int) {
	rr.status = status
	rr.w.WriteHeader(status)
}

func (rr *responseRecorder) Header() http.Header {
	return rr.w.Header()
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	return rr.w.Write(b)
}

type headerWriter struct {
	headers     map[string]string
	w           http.ResponseWriter
	wroteHeader bool
}

func newHeaderWriter(local bool, w http.ResponseWriter) *headerWriter {
	headers := map[string]string{
		"Content-Security-Policy:":          "default-src 'self'", // https://report-uri.com/home/generate
		"X-XSS-Protection":                  "1; mode=block",
		"X-Frame-Options":                   "sameorigin",
		"X-Content-Type-Options":            "nosniff",
		"X-Permitted-Cross-Domain-Policies": "none",
		"Referrer-Policy":                   "no-referrer-when-downgrade",
	}

	// TODO: local only
	if false {
		headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains; preload"
	}

	return &headerWriter{
		headers:     headers,
		w:           w,
		wroteHeader: false,
	}
}

func (hr *headerWriter) WriteHeader(status int) {
	// Other handlers did a w.WriteHeader(status).
	if !hr.wroteHeader {
		for k, v := range hr.headers {
			hr.w.Header().Set(k, v)
		}

		hr.wroteHeader = true
	}

	hr.w.WriteHeader(status)
}

func (hr *headerWriter) Header() http.Header {
	return hr.w.Header()
}

func (hr *headerWriter) Write(b []byte) (int, error) {
	// Other handlers didn't do a WriteHeader(status).
	if !hr.wroteHeader {
		for k, v := range hr.headers {
			hr.w.Header().Set(k, v)
		}

		hr.wroteHeader = true
	}

	return hr.w.Write(b)
}

type WebServer struct {
	mux    *http.ServeMux
	srv    *http.Server
	logger *slog.Logger
	subber *Subber
	assets http.FileSystem
}

func NewWebServer(logger *slog.Logger, subber *Subber) (*WebServer, error) {
	fsys, err := fs.Sub(fsAssets, "assets")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	hs := &http.Server{
		Addr:                         ":8081",
		Handler:                      mux,
		DisableGeneralOptionsHandler: false,
		ReadTimeout:                  10 * time.Second,
		ReadHeaderTimeout:            10 * time.Second,
		WriteTimeout:                 10 * time.Second,
		IdleTimeout:                  10 * time.Second,
		MaxHeaderBytes:               10 >> 10,
		TLSNextProto:                 nil,
		ConnState:                    nil,
		// ErrorLog:                     &log.Logger{},
		// BaseContext: func(net.Listener) context.Context {
		// },
		// ConnContext: func(ctx context.Context, c net.Conn) context.Context {
		// },
	}

	ws := &WebServer{
		mux:    mux,
		srv:    hs,
		logger: logger,
		subber: subber,
		assets: http.FS(fsys),
	}

	// attach routes to WebServer. This is a awkward compared to defining during
	// struct construction like `mc` but required in order for routes to have
	// access to private fields defined on the WebServer struct, such as loggers,
	// tracing, etc.
	ws.setRoutes()

	return ws, nil
}

// Run starts the HTTP Server application and gracefully shuts down when the
// provided context is marked done.
func (ws *WebServer) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", ws.srv.Addr)
	if err != nil {
		return err
	}

	// TODO: replace `[::]:8081` with http://localhost:8081 or something clickable.
	ws.logger.Info(fmt.Sprintf("listening on: %s", ln.Addr().String()))

	var group errgroup.Group

	group.Go(func() error {
		<-ctx.Done()

		// before shutting down the HTTP server wait for any HTTP requests that are
		// in transit on the network. Common in Kubernetes and other distributed
		// systems.
		time.Sleep(httpShutdownPreStopDelaySeconds * time.Second)

		// give active connections time to complete or disconnect before closing.
		drainTimeoutCtx, cancel := context.WithTimeout(ctx, httpShutdownTimeoutSeconds*time.Second)
		defer cancel()

		return ws.srv.Shutdown(drainTimeoutCtx)
	})

	group.Go(func() error {
		err := ws.srv.Serve(ln)
		// http.ErrServerClosed is expected at shutdown.
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	})

	return group.Wait()
}

func (ws *WebServer) respondError(status int, err error, w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(status)

	ws.logger.Error("respondError()", slog.String("error", err.Error()))
}

func (ws *WebServer) renderTemplate(status int, t templ.Component, w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(status)

	if err := t.Render(r.Context(), w); err != nil {
		ws.logger.Error("t.Render()", slog.String("error", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (ws *WebServer) middlewareChain(next http.Handler) http.HandlerFunc {
	return ws.securityMiddleware(
		ws.loggingMiddleware(
			ws.corsMiddleware(
				func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r)
				},
			)))
}

// corsMiddleware responds to OPTION requests and injects CORS headers when required.
// See: https://bunrouter.uptrace.dev/guide/golang-cors.html
func (ws *WebServer) corsMiddleware(next func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next(w, r)

			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET,PUT,POST,DELETE,HEAD")
			w.Header().Set("Access-Control-Allow-Headers", "authorization,content-type,content-length")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)

			return
		}

		next(w, r)
	}
}

func (ws *WebServer) securityMiddleware(next func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO, get local from env
		hr := newHeaderWriter(true, w)
		next(hr, r)

		// Other handlers didn't WriteHeader(status) or Write(b).
		if !hr.wroteHeader {
			for k, v := range hr.headers {
				hr.w.Header().Set(k, v)
			}

			hr.wroteHeader = true
		}
	}
}

func (ws *WebServer) loggingMiddleware(next func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rr := &responseRecorder{
			w:      w,
			status: http.StatusOK, // default, handlers will override if need.
		}

		start := time.Now()

		next(rr, r)

		ws.logger.WithGroup("request").LogAttrs(
			r.Context(), slog.LevelInfo.Level(), "http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rr.status),
			slog.Int64("duration_us", time.Since(start).Microseconds()),
		)
	}
}

// HandleStaticFiles is a HTTP handler for static files available in the
// embedded filesystem set in NewWebServer().
func (ws *WebServer) HandleStaticFiles() http.HandlerFunc {
	fs := http.FileServer(ws.assets)

	return func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(r.URL.Redacted())

		f, err := ws.assets.Open(path)
		if err != nil {
			http.NotFoundHandler().ServeHTTP(w, r)

			return
		}

		stat, err := f.Stat()
		if err != nil {
			http.NotFoundHandler().ServeHTTP(w, r)

			return
		}

		if stat.IsDir() {
			http.NotFoundHandler().ServeHTTP(w, r)

			return
		}

		closeErr := f.Close()
		if closeErr != nil {
			http.NotFoundHandler().ServeHTTP(w, r)

			return
		}

		fs.ServeHTTP(w, r)
	}
}
