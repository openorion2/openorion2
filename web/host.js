// The page side of the browser host (internal/host/web.go): the canvas,
// mouse and keyboard events, and Web Audio. Go drives it once a frame
// through the global openorion2Host.
"use strict";

(() => {
	const W = 640, H = 480, RATE = 44100;
	const LEAD = 0.12; // seconds of sound kept scheduled ahead

	// KeyboardEvent.code -> [USB HID usage, SDL keycode], as SDL reports
	// keys: printable ones by their unshifted character.
	const keys = {
		Enter: [40, 13], Escape: [41, 27], Backspace: [42, 8], Tab: [43, 9], Space: [44, 32],
		Minus: [45, 45], Equal: [46, 61], BracketLeft: [47, 91], BracketRight: [48, 93], Backslash: [49, 92],
		Semicolon: [51, 59], Quote: [52, 39], Backquote: [53, 96], Comma: [54, 44], Period: [55, 46], Slash: [56, 47],
		Insert: [73], Home: [74], PageUp: [75], Delete: [76, 127], End: [77], PageDown: [78],
		ArrowRight: [79], ArrowLeft: [80], ArrowDown: [81], ArrowUp: [82],
		NumpadDivide: [84], NumpadMultiply: [85], NumpadSubtract: [86], NumpadAdd: [87], NumpadEnter: [88],
		NumpadDecimal: [99],
	};
	for (let i = 0; i < 26; i++) keys["Key" + String.fromCharCode(65 + i)] = [4 + i, 97 + i];
	for (let i = 1; i <= 9; i++) keys["Digit" + i] = [29 + i, 48 + i];
	keys.Digit0 = [39, 48];
	for (let i = 1; i <= 12; i++) keys["F" + i] = [57 + i];
	for (let i = 1; i <= 9; i++) keys["Numpad" + i] = [88 + i];
	keys.Numpad0 = [98];

	const canvas = document.getElementById("screen");
	const ctx = canvas.getContext("2d", { alpha: false });
	const src = document.createElement("canvas");
	src.width = W;
	src.height = H;
	const srcCtx = src.getContext("2d", { alpha: false });
	const img = new ImageData(W, H);

	let events = [];
	let rafId = 0;
	let audio = null, nextTime = 0, lastNeed = performance.now();

	function toScreen(ev) {
		const r = canvas.getBoundingClientRect();
		return {
			x: Math.floor(((ev.clientX - r.left) / r.width) * W),
			y: Math.floor(((ev.clientY - r.top) / r.height) * H),
		};
	}
	const buttonBit = (b) => (b === 0 ? 1 : b === 2 ? 2 : 0);

	function startAudio() {
		if (!audio) {
			try {
				audio = new AudioContext({ sampleRate: RATE });
			} catch (e) {
				audio = new AudioContext();
			}
		}
		if (audio.state !== "running") audio.resume();
	}

	canvas.addEventListener("pointermove", (ev) => {
		const p = toScreen(ev);
		events.push({ t: "move", x: p.x, y: p.y });
	});
	canvas.addEventListener("pointerdown", (ev) => {
		startAudio();
		canvas.setPointerCapture(ev.pointerId);
		const p = toScreen(ev);
		events.push({ t: "down", x: p.x, y: p.y, b: buttonBit(ev.button) });
		ev.preventDefault();
	});
	canvas.addEventListener("pointerup", (ev) => {
		const p = toScreen(ev);
		events.push({ t: "up", x: p.x, y: p.y, b: buttonBit(ev.button) });
	});
	canvas.addEventListener("contextmenu", (ev) => ev.preventDefault());
	window.addEventListener("keydown", (ev) => {
		startAudio();
		if (ev.metaKey) return; // leave the browser's own shortcuts alone
		if (ev.altKey && ev.code === "Enter") {
			if (document.fullscreenElement) document.exitFullscreen();
			else document.documentElement.requestFullscreen();
			ev.preventDefault();
			return;
		}
		const k = keys[ev.code];
		if (!k) return;
		ev.preventDefault();
		events.push({
			t: "key", hid: k[0], code: k.length > 1 ? k[1] : (k[0] | 0x40000000) >>> 0,
			shift: ev.shiftKey, ctrl: ev.ctrlKey, alt: ev.altKey,
		});
	});

	// layout sizes the canvas to the largest 4:3 box in the window. It holds
	// the frame enlarged by a whole factor with nearest neighbour, which the
	// browser then scales smoothly to the box ("sharp bilinear").
	function layout() {
		const cw = Math.min(window.innerWidth, (window.innerHeight * 4) / 3);
		const ch = (cw * 3) / 4;
		const k = Math.max(1, Math.ceil((cw * window.devicePixelRatio) / W));
		if (canvas.width !== W * k) {
			canvas.width = W * k;
			canvas.height = H * k;
		}
		canvas.style.width = cw + "px";
		canvas.style.height = ch + "px";
	}

	globalThis.openorion2Host = {
		pixels: img.data,

		start(onFrame) {
			const loop = () => {
				onFrame();
				rafId = requestAnimationFrame(loop);
			};
			rafId = requestAnimationFrame(loop);
		},
		stop() {
			cancelAnimationFrame(rafId);
		},
		drain() {
			const e = events;
			events = [];
			return e;
		},
		present() {
			layout();
			srcCtx.putImageData(img, 0, 0);
			ctx.imageSmoothingEnabled = false;
			ctx.drawImage(src, 0, 0, canvas.width, canvas.height);
		},

		// audioNeed is how many sample frames to mix now. Until the page may
		// play sound they are mixed and dropped at the real-time rate.
		audioNeed() {
			const now = performance.now();
			if (!audio || audio.state !== "running") {
				const n = Math.round(((now - lastNeed) / 1000) * RATE);
				lastNeed = now;
				return Math.min(n, RATE / 5);
			}
			lastNeed = now;
			const t = audio.currentTime;
			if (nextTime < t) nextTime = t + 0.02;
			return Math.max(0, Math.floor((LEAD - (nextTime - t)) * RATE));
		},
		// audioPush plays n frames of 16-bit little-endian stereo.
		audioPush(bytes, n) {
			if (!audio || audio.state !== "running") return;
			const s = new Int16Array(bytes.buffer, 0, 2 * n);
			const buf = audio.createBuffer(2, n, RATE);
			const l = buf.getChannelData(0), r = buf.getChannelData(1);
			for (let i = 0; i < n; i++) {
				l[i] = s[2 * i] / 32768;
				r[i] = s[2 * i + 1] / 32768;
			}
			const node = audio.createBufferSource();
			node.buffer = buf;
			node.connect(audio.destination);
			node.start(nextTime);
			nextTime += n / RATE;
		},
	};
})();
