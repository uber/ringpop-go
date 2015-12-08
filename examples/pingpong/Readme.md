 Run the example
-----------------

Terminal 1:
```bash
go run pingpong.go
```

Terminal 2:
```bash
tcurl -p 127.0.0.1:3000 -t pingpong.thrift pingpong PingPong::Ping -3 '{"request":{"key":"hello"}}'
```

 Update the generated code
---------------------------

Make sure you have the `thrift` and `thrift-gen` binaries installed on your path.
Look at the Readme.md in the root of this project on how to do this.

```bash
make generate
```