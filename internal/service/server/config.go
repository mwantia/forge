package server

type ServerConfig struct {
	Address string `hcl:"address,optional"`
	Token   string `hcl:"token,optional"`
}
