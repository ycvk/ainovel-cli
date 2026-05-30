package ainovelcli

import _ "embed"

// ConfigExampleJSONC is the canonical commented configuration template shipped
// with the CLI and copied to ~/.ainovel/config.example.jsonc during setup.
//
//go:embed config.example.jsonc
var ConfigExampleJSONC string
