// The file system Go's js/wasm runtime sees (the subset of Node's fs API that
// syscall/fs_js.go calls). There is one directory, /game: the original's
// files are read from the server on demand (whole or by HTTP range), and
// whatever the game writes (saves, MOX.SET, HOF.M2) lives in IndexedDB.
// Names are case-insensitive, as on the DOS and macOS file systems.
"use strict";

(() => {
	const O = { O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_EXCL: 128, O_TRUNC: 512, O_APPEND: 1024, O_DIRECTORY: 65536 };
	const DIR_MODE = 0o40755, FILE_MODE = 0o100644;
	// Base files up to this size, or read mostly at once, are fetched whole
	// and kept; larger ones are read by range (music, videos).
	const WHOLE = 4 << 20;
	const CACHE_BUDGET = 256 << 20;

	function fsErr(code) {
		const e = new Error(code);
		e.code = code;
		return e;
	}

	function stat(isDir, size, mtimeMs) {
		return {
			dev: 0, ino: 0, mode: isDir ? DIR_MODE : FILE_MODE, nlink: 1, uid: 0, gid: 0, rdev: 0,
			size, blksize: 4096, blocks: Math.ceil(size / 512),
			atimeMs: mtimeMs, mtimeMs, ctimeMs: mtimeMs,
			isDirectory: () => isDir,
		};
	}

	// The IndexedDB store of written files: key the upper-case name, value
	// {name, data, mtime} or {name, deleted: true}.
	class Store {
		async open() {
			this.db = await new Promise((resolve, reject) => {
				const r = indexedDB.open("openorion2", 1);
				r.onupgradeneeded = () => r.result.createObjectStore("files");
				r.onsuccess = () => resolve(r.result);
				r.onerror = () => reject(r.error);
			});
		}
		all() {
			return new Promise((resolve, reject) => {
				const out = new Map();
				const r = this.db.transaction("files").objectStore("files").openCursor();
				r.onsuccess = () => {
					const c = r.result;
					if (!c) return resolve(out);
					out.set(c.key, c.value);
					c.continue();
				};
				r.onerror = () => reject(r.error);
			});
		}
		put(key, value) {
			return new Promise((resolve, reject) => {
				const t = this.db.transaction("files", "readwrite");
				t.objectStore("files").put(value, key);
				t.oncomplete = () => resolve();
				t.onerror = () => reject(t.error);
			});
		}
		clear() {
			return new Promise((resolve, reject) => {
				const t = this.db.transaction("files", "readwrite");
				t.objectStore("files").clear();
				t.oncomplete = () => resolve();
				t.onerror = () => reject(t.error);
			});
		}
	}

	class GameFS {
		constructor() {
			this.constants = O;
			this.files = new Map(); // upper-case name -> entry
			this.fds = new Map();
			this.nextFd = 100;
			this.cached = 0; // bytes of base files kept whole
			this.out = { 1: "", 2: "" };
			this.decoder = new TextDecoder("utf-8");
		}

		// init reads the server's list of game files and the saved ones.
		async init(baseURL, manifestURL) {
			this.base = baseURL;
			const list = await (await fetch(manifestURL, { cache: "no-cache" })).json();
			for (const f of list) {
				this.files.set(f.name.toUpperCase(), { name: f.name, size: f.size, mtime: f.mtime, base: true, data: null, used: 0 });
			}
			this.store = new Store();
			await this.store.open();
			for (const [key, v] of await this.store.all()) {
				if (v.deleted) {
					this.files.delete(key);
					continue;
				}
				this.files.set(key, { name: v.name, size: v.data.length, mtime: v.mtime, base: false, data: v.data, used: 0 });
			}
		}

		// written lists the files the game wrote, for export.
		written() {
			return [...this.files.values()].filter((e) => !e.base);
		}

		// resolve turns a path into {dir: true} or {key, name} for a file in /game.
		resolve(path) {
			const parts = [];
			for (const p of path.split("/")) {
				if (p === "" || p === ".") continue;
				if (p === "..") parts.pop();
				else parts.push(p);
			}
			if (parts.length === 0) return { dir: "/" };
			if (parts[0].toUpperCase() !== "GAME") return null;
			if (parts.length === 1) return { dir: "/game" };
			if (parts.length === 2) return { key: parts[1].toUpperCase(), name: parts[1] };
			return null;
		}

		async fetchWhole(e) {
			const r = await fetch(this.base + encodeURIComponent(e.name));
			if (!r.ok) throw fsErr("EIO");
			const data = new Uint8Array(await r.arrayBuffer());
			e.data = data;
			if (e.base) {
				this.cached += data.length;
				this.evict(e);
			}
			return data;
		}

		async fetchRange(e, from, to) {
			const r = await fetch(this.base + encodeURIComponent(e.name), { headers: { Range: `bytes=${from}-${to - 1}` } });
			if (!r.ok) throw fsErr("EIO");
			const data = new Uint8Array(await r.arrayBuffer());
			return r.status === 206 ? data : data.subarray(from, to);
		}

		// evict drops the least recently used whole base files over the budget.
		evict(keep) {
			if (this.cached <= CACHE_BUDGET) return;
			const held = new Set([...this.fds.values()].map((f) => f.e));
			const lru = [...this.files.values()].filter((e) => e.base && e.data && e !== keep && !held.has(e)).sort((a, b) => a.used - b.used);
			for (const e of lru) {
				if (this.cached <= CACHE_BUDGET) break;
				this.cached -= e.data.length;
				e.data = null;
			}
		}

		async persist(e) {
			if (e.base) this.cached -= e.data.length;
			e.base = false;
			await this.store.put(e.name.toUpperCase(), { name: e.name, data: e.data.slice(0, e.size), mtime: e.mtime });
		}

		run(cb, f) {
			f().then((v) => cb(null, v), (err) => cb(err.code ? err : fsErr("EIO")));
		}

		writeSync(fd, buf) {
			if (fd === 1 || fd === 2) {
				this.out[fd] += this.decoder.decode(buf);
				const nl = this.out[fd].lastIndexOf("\n");
				if (nl !== -1) {
					(fd === 1 ? console.log : console.error)(this.out[fd].substring(0, nl));
					this.out[fd] = this.out[fd].substring(nl + 1);
				}
				return buf.length;
			}
			throw fsErr("EBADF");
		}

		open(path, flags, mode, cb) {
			this.run(cb, async () => {
				const r = this.resolve(path);
				if (!r) throw fsErr("ENOENT");
				const write = (flags & (O.O_WRONLY | O.O_RDWR)) !== 0;
				if (r.dir) {
					if (write) throw fsErr("EISDIR");
					const fd = this.nextFd++;
					this.fds.set(fd, { dir: r.dir });
					return fd;
				}
				if (flags & O.O_DIRECTORY) throw fsErr("ENOTDIR");
				let e = this.files.get(r.key);
				if (e && (flags & O.O_CREAT) && (flags & O.O_EXCL)) throw fsErr("EEXIST");
				if (!e) {
					if (!(flags & O.O_CREAT)) throw fsErr("ENOENT");
					e = { name: r.name, size: 0, mtime: Date.now(), base: false, data: new Uint8Array(0), used: 0 };
					this.files.set(r.key, e);
				}
				if (write && (flags & O.O_TRUNC)) {
					if (e.base && e.data) this.cached -= e.data.length;
					e.data = new Uint8Array(0);
					e.size = 0;
					e.mtime = Date.now();
					e.base = false;
					e.dirty = true;
				} else if (write && !e.data) {
					await this.fetchWhole(e);
				}
				const fd = this.nextFd++;
				this.fds.set(fd, { e, pos: 0, write, append: (flags & O.O_APPEND) !== 0 });
				return fd;
			});
		}

		close(fd, cb) {
			this.run(cb, async () => {
				const f = this.fds.get(fd);
				if (!f) throw fsErr("EBADF");
				this.fds.delete(fd);
				if (f.e && f.e.dirty) {
					f.e.dirty = false;
					await this.persist(f.e);
				}
			});
		}

		fsync(fd, cb) {
			this.run(cb, async () => {
				const f = this.fds.get(fd);
				if (f && f.e && f.e.dirty) {
					f.e.dirty = false;
					await this.persist(f.e);
				}
			});
		}

		read(fd, buf, offset, length, position, cb) {
			this.run(cb, async () => {
				const f = this.fds.get(fd);
				if (!f) throw fsErr("EBADF");
				if (f.dir) throw fsErr("EISDIR");
				const e = f.e;
				const pos = position === null ? f.pos : position;
				const n = Math.max(0, Math.min(length, e.size - pos));
				if (n > 0) {
					e.used = performance.now();
					if (!e.data && (e.size <= WHOLE || n >= e.size / 2)) await this.fetchWhole(e);
					const src = e.data ? e.data.subarray(pos, pos + n) : await this.fetchRange(e, pos, pos + n);
					buf.set(src, offset);
				}
				if (position === null) f.pos = pos + n;
				return n;
			});
		}

		write(fd, buf, offset, length, position, cb) {
			if (fd === 1 || fd === 2) {
				cb(null, this.writeSync(fd, buf.subarray(offset, offset + length)));
				return;
			}
			this.run(cb, async () => {
				const f = this.fds.get(fd);
				if (!f || !f.write) throw fsErr("EBADF");
				const e = f.e;
				const pos = f.append ? e.size : position === null ? f.pos : position;
				const end = pos + length;
				if (end > e.data.length) {
					const grown = new Uint8Array(Math.max(end, e.data.length * 2));
					grown.set(e.data.subarray(0, e.size));
					e.data = grown;
				}
				e.data.set(buf.subarray(offset, offset + length), pos);
				e.size = Math.max(e.size, end);
				e.mtime = Date.now();
				e.dirty = true;
				if (position === null) f.pos = end;
				return length;
			});
		}

		fstat(fd, cb) {
			const f = this.fds.get(fd);
			if (!f) return cb(fsErr("EBADF"));
			cb(null, f.dir ? stat(true, 0, 0) : stat(false, f.e.size, f.e.mtime));
		}

		stat(path, cb) {
			const r = this.resolve(path);
			if (!r) return cb(fsErr("ENOENT"));
			if (r.dir) return cb(null, stat(true, 0, 0));
			const e = this.files.get(r.key);
			if (!e) return cb(fsErr("ENOENT"));
			cb(null, stat(false, e.size, e.mtime));
		}

		lstat(path, cb) {
			this.stat(path, cb);
		}

		readdir(path, cb) {
			const r = this.resolve(path);
			if (!r) return cb(fsErr("ENOENT"));
			if (!r.dir) return cb(fsErr("ENOTDIR"));
			cb(null, r.dir === "/" ? ["game"] : [...this.files.values()].map((e) => e.name));
		}

		ftruncate(fd, length, cb) {
			const f = this.fds.get(fd);
			if (!f || !f.write) return cb(fsErr("EBADF"));
			this.truncateEntry(f.e, length);
			cb(null);
		}

		truncate(path, length, cb) {
			this.run(cb, async () => {
				const r = this.resolve(path);
				const e = r && r.key && this.files.get(r.key);
				if (!e) throw fsErr("ENOENT");
				if (!e.data) await this.fetchWhole(e);
				this.truncateEntry(e, length);
				await this.persist(e);
			});
		}

		truncateEntry(e, length) {
			if (length > e.data.length) {
				const grown = new Uint8Array(length);
				grown.set(e.data.subarray(0, e.size));
				e.data = grown;
			} else {
				e.data.fill(0, length, e.size);
			}
			e.size = length;
			e.mtime = Date.now();
			e.dirty = true;
		}

		unlink(path, cb) {
			this.run(cb, async () => {
				const r = this.resolve(path);
				if (!r || !r.key || !this.files.has(r.key)) throw fsErr("ENOENT");
				const e = this.files.get(r.key);
				this.files.delete(r.key);
				await this.store.put(r.key, { name: e.name, deleted: true });
			});
		}

		rename(from, to, cb) {
			this.run(cb, async () => {
				const a = this.resolve(from), b = this.resolve(to);
				if (!a || !a.key || !this.files.has(a.key)) throw fsErr("ENOENT");
				if (!b || !b.key) throw fsErr("EACCES");
				const e = this.files.get(a.key);
				if (!e.data) await this.fetchWhole(e);
				this.files.delete(a.key);
				await this.store.put(a.key, { name: e.name, deleted: true });
				e.name = b.name;
				this.files.set(b.key, e);
				await this.persist(e);
			});
		}

		mkdir(path, perm, cb) {
			const r = this.resolve(path);
			cb(fsErr(r && r.dir ? "EEXIST" : "EACCES"));
		}
		rmdir(path, cb) { cb(fsErr("EACCES")); }
		chmod(path, mode, cb) { cb(null); }
		fchmod(fd, mode, cb) { cb(null); }
		chown(path, uid, gid, cb) { cb(null); }
		fchown(fd, uid, gid, cb) { cb(null); }
		lchown(path, uid, gid, cb) { cb(null); }
		utimes(path, atime, mtime, cb) { cb(null); }
		link(path, link, cb) { cb(fsErr("ENOSYS")); }
		symlink(path, link, cb) { cb(fsErr("ENOSYS")); }
		readlink(path, cb) { cb(fsErr("EINVAL")); }
	}

	globalThis.fs = new GameFS();
})();
