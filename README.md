# Homely API client

This is an unofficial API client for the [Homely](https://www.homely.no/) API. The API is in beta
at the time of writing. To connect use your email as the username and your Homely password as the password.

## Feature requests and issues are welcome!

## Usage

Download with:
```bash
go get github.com/tokongs/homely
```

See the usage example in [examples/main.go](./example/main.go).

It is currently possible to list your locations. Get a detailed snapshot of a given location
and its devices, and listening for update events. This is all the Homely API currently supports.

