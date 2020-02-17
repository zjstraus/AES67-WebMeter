This is a simple tool to find and receive an AES67 audio stream and present a websocket with live updates to draw meters on a webpage.

# Usage
Once built the following options are available when launching:

* interface: Network interface to listen on
* listinterfaces: List potential network interfaces and quit
* stream: AES67 stream name to receive
* sapadress: Override address to monitor for SAP announcements
* httpaddress: Address to bind to for HTTP/WS (default 0.0.0.0:8844)

It will attempt to discover and listen to the given stream, and will serve a webpage with basic live meters.

# Custom Meters
You can build your own meter graphics in any system that can mount a websocket.
The server presents a websocket at `/ws/live` and will send 30 JSON messages per second with new data to all connected clients.

## Websocket Messages
Each message is an array of objects, one for each channel in the received stream.

Each object has the following keys:
* RMS (number between -100 and 0) RMS level over the last 300ms in DBFS
* Peak (number between -100 and 0) Peak level in DBFS
* Latest (number between -100 and 0) Most recent level in DBFS