package main

import "embed"

//go:embed dist/*
var embeddedFrontend embed.FS
