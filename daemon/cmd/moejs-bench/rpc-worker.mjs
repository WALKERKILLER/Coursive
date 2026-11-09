// The process-plugin backend of moejs-bench: a Node worker that loads the
// pilot module and answers {id, action, input} JSON lines on stdin with
// {id, output} or {id, error} on stdout.
import { createInterface } from "node:readline";
import { pathToFileURL } from "node:url";

const pilot = await import(pathToFileURL(`${process.argv[2]}/index.js`).href);
process.stdout.write("ready\n");
for await (const line of createInterface({ input: process.stdin })) {
	const { id, action, input } = JSON.parse(line);
	let reply;
	try {
		reply = { id, output: pilot[action](input) };
	} catch (e) {
		reply = { id, error: String(e?.message ?? e) };
	}
	process.stdout.write(`${JSON.stringify(reply)}\n`);
}
