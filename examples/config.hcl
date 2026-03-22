# Example Forge Configuration
# Usage: forge agent --path examples/config.hcl

log_level  = "DEBUG"
plugin_dir = "./examples/plugins"

server {
    address = "127.0.0.1:9280"
    token   = ""
}

metrics {
    address = "127.0.0.1:9500"
}

# Plugin configurations
# Each plugin can have its own configuration block
plugin "skills" {
    config {
        path = "./examples/skills"
    }
}

plugin "ollama" {
    config {
        address = "http://127.0.0.1:11434"
        model   = "glm-5:cloud"
    }
}