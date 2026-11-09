// Review Lab: every bridge action answers with one result form, the deck lives
// in plugin storage, and the agent fills it with the make_cards tool.
const SAMPLE = [
	{ front: "什么是 TCP 三次握手？", back: "SYN、SYN-ACK、ACK：双方确认收发能力并交换初始序号。" },
	{ front: "进程和线程的区别？", back: "进程是资源分配的单位，线程是调度的单位；同一进程的线程共享地址空间。" },
	{ front: "B+ 树为何适合数据库索引？", back: "内部节点只存键、扇出大，叶子有序链接，范围查询和磁盘读取都友好。" },
];
const MAX_CARDS = 50;

const deckOf = async (coursive) => {
	const deck = await coursive.storage.get("deck");
	return Array.isArray(deck) && deck.length ? deck : SAMPLE;
};

const card = (deck, index, revealed) => ({
	index,
	revealed,
	front: deck[index].front,
	back: revealed ? deck[index].back : "（点「显示答案」）",
	progress: `${index + 1} / ${deck.length}`,
});

export function activate(coursive) {
	const started = (state) => typeof state.index === "number";
	const notStarted = { notification: { text: "先点「开始」", level: "warning" } };

	coursive.action("start", async (_args, { viewId }) => ({ statePatch: { viewId, patch: card(await deckOf(coursive), 0, false) } }));
	coursive.action("reveal", async (_args, { state, viewId }) => {
		if (!started(state)) return notStarted;
		const deck = await deckOf(coursive);
		return { statePatch: { viewId, patch: card(deck, Math.min(state.index, deck.length - 1), true) } };
	});
	coursive.action("next", async (args, { state, viewId }) => {
		if (!started(state)) return notStarted;
		const deck = await deckOf(coursive);
		const index = state.index + (args.step ?? 1);
		if (index >= deck.length) return { notification: { text: "这组卡片已复习完" } };
		return { statePatch: { viewId, patch: card(deck, index, false) } };
	});
	coursive.action("open", async () => ({ navigation: { route: "deck", title: "复习卡" } }));
	// A write: the bridge makes it a proposal, and this runs only after the user approves it.
	coursive.action("save", async (_args, { approved }) => {
		if (!approved) throw new Error("存为笔记需要你先批准");
		const deck = await deckOf(coursive);
		const body = deck.map((c, i) => `### ${i + 1}. ${c.front}\n\n${c.back}`).join("\n\n");
		const note = await coursive.daemon.put("/api/notes", { title: "复习卡", body });
		return { notification: { text: `已存为笔记「${note?.title ?? "复习卡"}」` } };
	});
}

export default function (pi, coursive) {
	pi.registerTool({
		name: "make_cards",
		label: "生成复习卡",
		description: "Replaces the Review Lab deck with question/answer cards. Use it when the user asks to turn course points into review cards; then tell them to open 复习卡.",
		parameters: {
			type: "object",
			properties: {
				cards: {
					type: "array",
					maxItems: MAX_CARDS,
					items: { type: "object", properties: { front: { type: "string" }, back: { type: "string" } }, required: ["front", "back"] },
				},
			},
			required: ["cards"],
		},
		execute: async (_id, { cards }) => {
			const deck = (Array.isArray(cards) ? cards : [])
				.slice(0, MAX_CARDS)
				.map((c) => ({ front: String(c.front ?? "").slice(0, 200), back: String(c.back ?? "").slice(0, 1000) }))
				.filter((c) => c.front && c.back);
			if (!deck.length) return { content: [{ type: "text", text: "没有可用的卡片：每张都需要 front 和 back。" }], details: {} };
			await coursive.storage.set("deck", deck);
			return { content: [{ type: "text", text: `已放入 ${deck.length} 张复习卡，用户可以打开「复习卡」页面开始。` }], details: { cards: deck.length } };
		},
	});
}
