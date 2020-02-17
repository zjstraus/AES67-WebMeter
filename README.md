This is a simple tool to find and receive an AES67 audio stream and present a websocket with live updates to draw meters on a webpage.

# Usage
Once built the following options are available when launching:

* interface: Network interface to listen on
* listinterfaces: List potential network interfaces and quit
* stream: AES67 stream name to receive
* sapadress: Override address to monitor for SAP announcements
* httpaddress: Address to bind to for HTTP/WS (default 0.0.0.0:8844)

It will attempt to discover and listen to the given stream, and will serve a webpage with basic live meters.