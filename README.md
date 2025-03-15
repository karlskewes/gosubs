# Go Subs

Manage subs for your sports game.

Run the app locally on a laptop or home server and then access from your mobile
phone at the game via Tailscale VPN or similar.

## Getting Started

Create your team:

```
cp ./config_example.json config.json
vim ./config.json
```

Start server:

```
go run .
```

[View app](http://localhost:8081/)

Actions:

1. **Start** a game to begin the game timer and the first 'period'.
1. **Sub** On/Off players as need. Players play count and duration will increase.
1. **Pause** the game to sub off all players, for example at the end of a period/half.
   Then start the next period by clicking the **Resume** button.
1. **End** a game to stop the game timer and sub off all players.
1. **Reset** the game to start a new game, resetting player statistics.

## Contributing

Currently this project is feature complete for my use case.

However there are many directions this project could take. Please feel free to
fork or create an issue.

When developing open a separate terminal and run [`air`](https://github.com/air-verse/air).
This will regenerate CSS, render `<file>.templ` to `<file>.go` and restart the
app whenever a change is made in your IDE.

```
$ air
```

## Thanks

Creators:

- [Templ](https://templ.guide/)
- [Air](https://github.com/air-verse/air)
- [MariaLetta](https://github.com/MariaLetta/free-gophers-pack) for the Gopher Trophy/Cup SVG.
