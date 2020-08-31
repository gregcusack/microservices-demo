module github.com/triplewy/microservices-demo/src/productcatalogservice

go 1.14

require (
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.3.0
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	go.opencensus.io v0.21.0
	go.uber.org/zap v1.14.1
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7
	golang.org/x/sys v0.0.0-20190626221950-04f50cda93cb // indirect
	golang.org/x/text v0.3.2 // indirect
	google.golang.org/genproto v0.0.0-20190708153700-3bdd9d9f5532 // indirect
	google.golang.org/grpc v1.22.0
	gopkg.in/yaml.v2 v2.2.4 // indirect
)

replace git.apache.org/thrift.git v0.12.1-0.20190708170704-286eee16b147 => github.com/apache/thrift v0.12.1-0.20190708170704-286eee16b147
