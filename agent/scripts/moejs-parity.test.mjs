// The MoeJS pilot module in Node: its results are the golden file the Go
// executor test compares the MoeJS engine against
// (daemon/internal/moejsexec/factory_test.go), so the same input gives the
// same result on both. COURSIVE_UPDATE_GOLDEN=1 rewrites the file.
import assert from "node:assert/strict";
import { readFile, writeFile } from "node:fs/promises";
import test from "node:test";

const dir = new URL("../../daemon/internal/moejsexec/testdata/pilot/", import.meta.url);

export async function pilotResults() {
	const pilot = await import(new URL("index.js", dir));
	const { cases } = JSON.parse(await readFile(new URL("cases.json", dir), "utf8"));
	const out = {};
	for (const c of cases) {
		const input = c.inputFile ? { ...JSON.parse(await readFile(new URL(c.inputFile, dir), "utf8")), ...c.merge } : c.input;
		out[c.name] = pilot[c.action](input);
	}
	return out;
}

test("Node gives the pilot's golden results", async () => {
	const got = await pilotResults();
	const golden = new URL("expected.json", dir);
	if (process.env.COURSIVE_UPDATE_GOLDEN === "1") await writeFile(golden, `${JSON.stringify(got, null, "\t")}\n`);
	assert.deepEqual(got, JSON.parse(await readFile(golden, "utf8")));
});
