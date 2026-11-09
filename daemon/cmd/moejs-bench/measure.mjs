// Shared timing for the Node halves of moejs-bench: the first call, then n
// timed calls, with the same percentile rule as main.go.
export async function measure(name, n, call) {
	const row = { task: name, first_action_ms: 0, invoke_p50_ms: 0, invoke_p95_ms: 0, calls: n + 1, errors: 0 };
	const once = async () => {
		try {
			await call();
		} catch (e) {
			row.errors++;
			row.last_error = String(e?.message ?? e);
		}
	};
	let start = performance.now();
	await once();
	row.first_action_ms = performance.now() - start;
	const d = [];
	for (let i = 0; i < n; i++) {
		start = performance.now();
		await once();
		d.push(performance.now() - start);
	}
	d.sort((a, b) => a - b);
	if (n > 0) {
		row.invoke_p50_ms = d[Math.floor(((n - 1) * 50) / 100)];
		row.invoke_p95_ms = d[Math.floor(((n - 1) * 95) / 100)];
	}
	return row;
}
