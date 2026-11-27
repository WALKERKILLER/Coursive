import assert from "node:assert/strict";
import {test} from "node:test";
import {createTools} from "./tools.mjs";
import {runOp} from "./operate.mjs";

test("personal schedule and note edits show the change, revalidate and reverse it", async () => {
	const item={id:"s",source:"user",kind:"exam",course_id:"c",title:"考试",due_date:"2026-12-01",due_time:"",done:false};
	const note={id:"n",source:"user",title:"讲义",course_id:"c",body:"旧段落"};
	const originalFetch=globalThis.fetch;
	globalThis.fetch=async url=>Response.json(url.endsWith("/schedule")?[item]:note);
	const conv={course_id:"c",lastUser:"修改讲义，调整考试日程"};
	const tools=createTools(conv,{}, {act:async p=>({...p,content:[]})});
	const tool=name=>tools.find(t=>t.name===name);
	try {
		const edit=await tool("update_schedule").execute("1",{id:"s",due_date:"2026-12-05"});
		assert.equal(edit.forceConfirm,true);
		const deps={api:async(method,path,patch)=>method==="GET"?[item]:Object.assign(item,patch)};
		await runOp(edit.op,deps);
		assert.equal(item.due_date,"2026-12-05");
		item.due_date="2026-12-09";
		await assert.rejects(runOp(edit.undo,deps),/日程已变化/);
		item.due_date="2026-12-05";
		await runOp(edit.undo,deps);
		assert.equal(item.due_date,"2026-12-01");
		const save=await tool("save_note").execute("2",{note_id:"n",replace:true,title:"讲义",body:"新段落"});
		assert.equal(save.forceConfirm,true);
		assert.deepEqual(save.lines,["讲义","− 旧段落","+ 新段落"]);
		assert.equal(save.undo.note.body,"旧段落");
		assert.equal(save.undo.note.base_body,"新段落");
		const fresh=await tool("save_note").execute("3",{body:"新",title:"新笔记"});
		assert.equal(fresh.forceConfirm,true);
		assert.match(fresh.lines.join(" "),/无法.*撤销/);
	} finally {globalThis.fetch=originalFetch;}
});
