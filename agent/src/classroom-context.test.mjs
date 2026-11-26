import assert from "node:assert/strict";
import test from "node:test";
import { ClassroomContext } from "./classroom-context.mjs";

test("material inventory and transcript search stay within the active course", async () => {
  const calls = [];
  const responses = {
    "/api/lectures/L": {course:{id:"C"},lecture:{course_id:"C"}},
    "/api/courses/C/lectures": {lectures:[{id:"L",segments:0},{id:"N",segments:1}]},
    "/api/library/catalog?course_id=C": [
      {course_id:"C",lecture_id:"L",file_type:"md",name:"复习讲义"},
      {course_id:"C",file_type:"pptx",name:"Process"},
      {course_id:"other",file_type:"pptx",name:"私人课件"},
    ],
    "/api/lectures/N/transcript": [{start_ms:1250,end_ms:2900,text:"进程共享地址空间。"}],
  };
  const context = new ClassroomContext(async (_,url) => {
    calls.push(url); assert.ok(Object.hasOwn(responses,url),url); return responses[url];
  },"L",async (_,sources)=>sources);
  const inventory = await context.inventory();
  assert.deepEqual(inventory.shared_slides.map(f=>f.name),["Process"]);
  assert.deepEqual(inventory.materials.map(f=>f.name),["复习讲义"]);
  await assert.rejects(context.find("进程","进程","other"),/不属于/);
  const [hit] = await context.find("进程","进程");
  assert.equal(hit.lectureId,"N");
  assert.equal(hit.segments[0].startMs,1250);
  await context.find("进程","进程");
  assert.equal(calls.filter(p=>p.endsWith("/transcript")).length,1);
});
