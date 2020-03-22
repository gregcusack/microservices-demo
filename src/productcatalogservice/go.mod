module github.com/triplewy/microservices-demo/src/productcatalogservice

go 1.14

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.0
	github.com/GoogleCloudPlatform/microservices-demo v0.1.1
	github.com/golang/protobuf v1.3.1
	github.com/google/go-cmp v0.3.0
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/sirupsen/logrus v1.4.2
	go.opencensus.io v0.21.0
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7
	golang.org/x/sys v0.0.0-20190626221950-04f50cda93cb // indirect
	google.golang.org/api v0.7.1-0.20190709010654-aae1d1b89c27 // indirect
	google.golang.org/genproto v0.0.0-20190708153700-3bdd9d9f5532 // indirect
	google.golang.org/grpc v1.22.0
)

replace git.apache.org/thrift.git v0.12.1-0.20190708170704-286eee16b147 => github.com/apache/thrift v0.12.1-0.20190708170704-286eee16b147
