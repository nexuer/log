# log
This structured logging library is developed with reference to Go's standard log/slog package. Although slog provides a solid foundation for structured logging, it does not support delayed field loading with the With method, resulting in premature evaluation of dynamic values. This reduces flexibility and efficiency in scenarios where log attributes—such as timestamps, request IDs, or metrics—require runtime computation. To address this, we rewrote this library.
## Install
```shell
go get github.com/nexuer/log
```