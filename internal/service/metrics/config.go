package metrics

type MetricsConfig struct {
	Address string `hcl:"address,optional"`
	Token   string `hcl:"token,optional"`
}
