# until we have a makefile:

(cd meshservice ; protoc --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative  --go_out=. --go-grpc_out=. meshservice.proto )
