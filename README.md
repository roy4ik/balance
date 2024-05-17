# Balance
A Simple and Effective Go Software Load Balancer

Balance is built as a simple to use Load Balancer that disperses Requests to the server on which it runs as a proxy to the servers provided in its configuration.

The Slb can be used both as a handler on instances of http.Server

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
