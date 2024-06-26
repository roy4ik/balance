# Balance
A Simple and Effective Go Software Load Balancer

# Usage
Balance is built as a simple to use Load Balancer that disperses Requests to the server on which it runs as a proxy to the servers provided in its configuration.
Balance can be used as binary packed in a docker, or used directly from your code utilizing [`services/slb`](#Services/slb).
It itself runs a Grpc server to control and configure the load balancer (see api reference ./services/slb/api.proto)

# Services/slb
The Slb can be used as a handler on instances of http.Server

```mermaid
flowchart TD
ClientServer[Client]
FrontendServer[`FrontendServer: 
Handler: Slb]
ClientServer ~~~ FrontendServer
Start[/New/]
clientRequest[Request]
FrontendServeHTTP[/ServeHTTP/]
serve[/ServerHTTP/]
backendResponse[Response]
select[/Select/]

Start -->|`validate config, 
and set endpoints`| FrontendServer
ClientServer --> clientRequest --> FrontendServer

FrontendServer --> FrontendHandler[Handler] --> FrontendServeHTTP
FrontendServeHTTP --> Selector --> select --> serve -->|send to backend| BackendServer --> backendResponse -...-> ClientServer
```

# Builds 
## Docker
To build the Docker image run
```make balance-docker```