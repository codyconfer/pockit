package data

import _ "embed"

//go:embed sources.json
var Catalog []byte

//go:embed sources.schema.json
var Schema []byte

//go:embed roles.json
var Roles []byte
