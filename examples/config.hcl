# Example Forge Configuration
# Usage: forge agent --path examples/config.hcl

log_level  = "DEBUG"
plugin_dir = "./examples/plugins"

storage "file" {
    path = "./data"
}

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
    # Optional
    runtime {
        path    = "/path/to/plugin"
        args    = ["arg0", "arg1", "argN"]
        timeout = "60s"
        
        port {
            min = 12000
            max = 20000
        }

        env {
            foo = "bar"
        }
    }

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