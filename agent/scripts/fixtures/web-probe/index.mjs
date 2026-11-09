// The probe's actions: a read, a report sink for the page's findings, and a
// write that must never run without the user's approval.
export function activate(coursive) {
	coursive.action("cards", async () => ({ notification: { text: "3 张卡片" } }));
	coursive.action("report", async () => ({ notification: { text: "收到" } }));
	coursive.action("save", async (_args, { approved }) => ({ notification: { text: `已保存 ${approved}` } }));
}
