package server

type ServerConfig struct {
	Address string         `hcl:"address,optional"`
	Token   string         `hcl:"token,optional"`
	Swagger *SwaggerConfig `hcl:"swagger,block"`
}

// SwaggerConfig controls the embedded Swagger UI.
// Disabled by default — set enabled = true for development.
//
//	swagger {
//	  enabled = true
//	  path    = "/swagger"   # optional, defaults to /swagger
//	}
type SwaggerConfig struct {
	Path string `hcl:"path,optional"`
}
