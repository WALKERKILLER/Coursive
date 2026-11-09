// Hostile and boundary scripts for the executor's isolation tests.
import { shout } from "text";

export function spin() {
	for (;;) {}
}

export function grow() {
	const keep = [];
	for (let i = 0; ; i++) keep.push({ i, text: `第 ${i} 段` });
}

// The limit hit inside String.prototype.repeat is a RangeError the script can
// catch; the executor fails the call anyway.
export function growCaught() {
	const keep = [];
	try {
		for (let i = 0; ; i++) keep.push(`${i}`.repeat(256));
	} catch {
		return "caught";
	}
}

export function probe() {
	const kinds = {};
	for (const name of ["fetch", "require", "process", "XMLHttpRequest", "WebSocket", "setTimeout", "Deno", "Bun", "module"]) {
		kinds[name] = typeof globalThis[name];
	}
	return { kinds, coursive: Object.keys(coursive).sort(), library: typeof coursive.library };
}

export async function importFs() {
	return await import("fs");
}

export function evalCode() {
	return eval("1 + 1");
}

export function newFunction() {
	return new Function("return 1")();
}

export function readLibrary(input) {
	return coursive.library.read(input.path);
}

export function now() {
	return coursive.clock();
}

export function big() {
	return "x".repeat(2 << 20);
}

export async function settled(input) {
	const value = await Promise.resolve(input.n * 2);
	return { value, loud: shout("ok") };
}

export function fails() {
	throw new TypeError("坏输入");
}
