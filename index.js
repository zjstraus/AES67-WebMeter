let handleWebsocketMessage = function(){
    let table = document.getElementById("messages");
    let canvas = document.getElementById("meter");
    let height = canvas.height;
    let width = canvas.width;
    let canvasCtx = canvas.getContext("2d");
    let gradient = canvasCtx.createLinearGradient(0, 0, 0, 600);
    gradient.addColorStop(0, 'red');
    gradient.addColorStop(-15/-100, 'yellow');
    gradient.addColorStop(1, 'green')
    let DBtoHeight = function(x) {
        return ((100 + x) / 100) * height;
    };
    dataSocket.onmessage = function (event) {
        let data = JSON.parse(event.data);
        // Clear the canvas
        canvasCtx.fillStyle = "#333333";
        canvasCtx.fillRect(0,0,width,height);
        canvasCtx.fillStyle = gradient;
        let channels = data.length;
        let channelWidth = width / channels;
        for (let i = 0; i < channels; i++) {
            let channelHeight = DBtoHeight(data[i].RMS)
            canvasCtx.fillRect(i * channelWidth, height - channelHeight, channelWidth, channelHeight);
        }
    }
};

