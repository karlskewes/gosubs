package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoadConfig_EmptyConfig(t *testing.T) {
	want := DefaultConfiguration()

	buf := bytes.NewBufferString("{}")

	got, err := loadConfig(buf)
	if err != nil {
		t.Errorf("failed to load empty config: %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("loadConfig(...) mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadConfig_OverrideField(t *testing.T) {
	want := DefaultConfiguration()
	want.Players = []Player{{Name: "kunio", Number: 86}}

	json := `
	{
	"Players": [{"name":"kunio","number":86}]
	}
	`

	buf := bytes.NewBufferString(json)

	got, err := loadConfig(buf)
	if err != nil {
		t.Errorf("failed to load empty config: %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("loadConfig(...) mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadConfig_ConfigJsonFile(t *testing.T) {
	want := Config{
		Players: []Player{
			{Name: "jane", Number: 1},
			{Name: "john", Number: 2},
			{Name: "steve", Number: 3},
			{Name: "mary", Number: 4},
			{Name: "bob", Number: 5},
		},
	}

	json, err := os.ReadFile("./config_example.json")
	if err != nil {
		t.Errorf("failed to read example config: %v", err)
	}

	buf := bytes.NewBuffer(json)

	got, err := loadConfig(buf)
	if err != nil {
		t.Errorf("failed to load empty config: %v", err)
	}

	// t.Logf("def: %#v", got)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("loadConfig(...) mismatch (-want +got):\n%s", diff)
	}
}
