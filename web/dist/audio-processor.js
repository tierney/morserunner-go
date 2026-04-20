class PCMProcessor extends AudioWorkletProcessor {
  constructor() {
    super();
    this.buffer = new Float32Array(32000); // 2 second buffer
    this.readPtr = 0;
    this.writePtr = 0;
    this.count = 0;

    this.port.onmessage = (event) => {
      const pcm = event.data; // Int16Array
      for (let i = 0; i < pcm.length; i++) {
        this.buffer[this.writePtr] = pcm[i] / 32768.0;
        this.writePtr = (this.writePtr + 1) % this.buffer.length;
        this.count++;
      }
    };
  }

  process(inputs, outputs, parameters) {
    const output = outputs[0];
    const channel = output[0];

    for (let i = 0; i < channel.length; i++) {
      if (this.count > 0) {
        channel[i] = this.buffer[this.readPtr];
        this.readPtr = (this.readPtr + 1) % this.buffer.length;
        this.count--;
      } else {
        channel[i] = 0;
      }
    }

    return true;
  }
}

registerProcessor('pcm-processor', PCMProcessor);
