//go:generate go run ../../tools/plugins -manifest ../../plugins.yaml -out .
//go:generate swag init -g swagger.go -d .,../../internal/service/server/api,../../internal/service/pipeline,../../internal/service/plugins,../../internal/service/tools -o ../../docs --parseInternal
package main
