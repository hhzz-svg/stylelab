/**
 * 沉浸式环境音效引擎 (Web Audio API 纯合成发生器，零外部音频资源依赖)
 */

export type AmbientSoundType = 'rain' | 'breeze' | 'cosmic'

class AmbientEngine {
  private ctx: AudioContext | null = null
  private gainNode: GainNode | null = null
  private activeType: AmbientSoundType | null = null
  private noiseNode: AudioNode | null = null
  private intervalId: number | null = null
  private currentVolume = 0.4

  private getContext(): AudioContext {
    if (!this.ctx) {
      const AudioCtx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext
      this.ctx = new AudioCtx()
    }
    if (this.ctx.state === 'suspended') {
      void this.ctx.resume()
    }
    return this.ctx
  }

  public play(type: AmbientSoundType, volume = this.currentVolume): void {
    this.stop()
    this.currentVolume = volume
    const ctx = this.getContext()

    const masterGain = ctx.createGain()
    masterGain.gain.setValueAtTime(volume, ctx.currentTime)
    masterGain.connect(ctx.destination)
    this.gainNode = masterGain
    this.activeType = type

    if (type === 'rain') {
      this.startRain(ctx, masterGain)
    } else if (type === 'breeze') {
      this.startBreeze(ctx, masterGain)
    } else if (type === 'cosmic') {
      this.startCosmic(ctx, masterGain)
    }
  }

  public setVolume(volume: number): void {
    this.currentVolume = Math.max(0, Math.min(1, volume))
    if (this.gainNode && this.ctx) {
      this.gainNode.gain.setTargetAtTime(this.currentVolume, this.ctx.currentTime, 0.05)
    }
  }

  public stop(): void {
    if (this.intervalId !== null) {
      window.clearInterval(this.intervalId)
      this.intervalId = null
    }
    if (this.noiseNode) {
      try {
        (this.noiseNode as unknown as { stop?: () => void }).stop?.()
        this.noiseNode.disconnect()
      } catch {
        // ignore
      }
      this.noiseNode = null
    }
    if (this.gainNode) {
      try {
        this.gainNode.disconnect()
      } catch {
        // ignore
      }
      this.gainNode = null
    }
    this.activeType = null
  }

  public getActiveType(): AmbientSoundType | null {
    return this.activeType
  }

  public isPlaying(): boolean {
    return this.activeType !== null
  }

  public getVolume(): number {
    return this.currentVolume
  }

  // --- Rain synthesizer (Pink noise + Bandpass drop pulses) ---
  private startRain(ctx: AudioContext, destination: AudioNode) {
    const bufferSize = ctx.sampleRate * 2
    const noiseBuffer = ctx.createBuffer(1, bufferSize, ctx.sampleRate)
    const output = noiseBuffer.getChannelData(0)
    let b0 = 0, b1 = 0, b2 = 0, b3 = 0, b4 = 0, b5 = 0, b6 = 0

    for (let i = 0; i < bufferSize; i++) {
      const white = Math.random() * 2 - 1
      b0 = 0.99886 * b0 + white * 0.0555179
      b1 = 0.99332 * b1 + white * 0.0750759
      b2 = 0.96900 * b2 + white * 0.1538520
      b3 = 0.86650 * b3 + white * 0.3104856
      b4 = 0.55000 * b4 + white * 0.5329522
      b5 = -0.7616 * b5 - white * 0.0168980
      output[i] = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + white * 0.5362) * 0.08
      b6 = white * 0.115926
    }

    const whiteNoise = ctx.createBufferSource()
    whiteNoise.buffer = noiseBuffer
    whiteNoise.loop = true

    const filter = ctx.createBiquadFilter()
    filter.type = 'lowpass'
    filter.frequency.setValueAtTime(1400, ctx.currentTime)

    whiteNoise.connect(filter)
    filter.connect(destination)
    whiteNoise.start(0)
    this.noiseNode = whiteNoise
  }

  // --- Breeze synthesizer (Deep brown noise + LFO filter sweep) ---
  private startBreeze(ctx: AudioContext, destination: AudioNode) {
    const bufferSize = ctx.sampleRate * 2
    const noiseBuffer = ctx.createBuffer(1, bufferSize, ctx.sampleRate)
    const output = noiseBuffer.getChannelData(0)
    let lastOut = 0.0

    for (let i = 0; i < bufferSize; i++) {
      const white = Math.random() * 2 - 1
      output[i] = (lastOut + (0.02 * white)) / 1.02
      lastOut = output[i]
      output[i] *= 1.5
    }

    const brownNoise = ctx.createBufferSource()
    brownNoise.buffer = noiseBuffer
    brownNoise.loop = true

    const filter = ctx.createBiquadFilter()
    filter.type = 'bandpass'
    filter.frequency.setValueAtTime(320, ctx.currentTime)
    filter.Q.setValueAtTime(1.2, ctx.currentTime)

    // LFO for sweeping wind rhythm
    const lfo = ctx.createOscillator()
    lfo.frequency.setValueAtTime(0.12, ctx.currentTime)
    const lfoGain = ctx.createGain()
    lfoGain.gain.setValueAtTime(180, ctx.currentTime)

    lfo.connect(lfoGain)
    lfoGain.connect(filter.frequency)
    lfo.start(0)

    brownNoise.connect(filter)
    filter.connect(destination)
    brownNoise.start(0)
    this.noiseNode = brownNoise
  }

  // --- Cosmic Drone synthesizer (Deep meditative multi-sine chords) ---
  private startCosmic(ctx: AudioContext, destination: AudioNode) {
    const freqs = [108, 162, 216, 324]
    const rootGain = ctx.createGain()
    rootGain.gain.setValueAtTime(0.25, ctx.currentTime)
    rootGain.connect(destination)

    freqs.forEach((f, idx) => {
      const osc = ctx.createOscillator()
      osc.type = idx % 2 === 0 ? 'sine' : 'triangle'
      osc.frequency.setValueAtTime(f + (Math.random() * 0.4 - 0.2), ctx.currentTime)

      const oscGain = ctx.createGain()
      oscGain.gain.setValueAtTime(0.3 / freqs.length, ctx.currentTime)

      osc.connect(oscGain)
      oscGain.connect(rootGain)
      osc.start(0)
    })

    this.noiseNode = rootGain
  }
}

export const ambient = new AmbientEngine()
