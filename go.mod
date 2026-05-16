module github.com/kave-io/go-sdk

go 1.26.1

require (
	connectrpc.com/connect v1.19.1
	github.com/kave-io/kave/proto/gen v0.0.0
)

require (
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250825161204-c5933d9347a5 // indirect
	google.golang.org/grpc v1.75.1 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

replace github.com/kave-io/kave/proto/gen => ../core/proto/gen
