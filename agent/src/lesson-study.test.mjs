import assert from "node:assert/strict";
import test from "node:test";
import {mkdtemp,rm} from "node:fs/promises";
import {join} from "node:path";
import {tmpdir} from "node:os";
import {lectureEvidence,lectureMaterials,runLessonStudy} from "./lesson-study.mjs";
import {MemoryStore,learningMemoryTool,memoryTools,hashOf} from "./memory.mjs";

test("sources belong to the current lecture and retain actual quote timing",async()=>{
 const call=async(_m,path)=>{
  assert.match(path,/^\/api\/lectures\/L\//);
  if(path.endsWith("/transcript"))return [{text:"第一段",start_ms:2000},{text:"第二段",start_ms:32000}];
  if(path.endsWith("/courseware"))return [];
  return [{idx:4}];
 };
 const {sources,coverage}=await lectureMaterials(call,"L",async()=>null);
 assert.deepEqual(sources[0].segments,[{at:0,seconds:2},{at:4,seconds:32}]);
 assert.match(coverage,/0 \/ 1/);
});

test("topic lookup reads full-lecture matches and opens only scoped files and frames",async()=>{
 const prefix="/api/lectures/L";
 const doc="课程/本节/带 空格课件.pdf", exam="课程/本节/2024真题.pdf";
 const rows=Array.from({length:35},(_,i)=>({text:"原始课堂内容 ".repeat(73),start_ms:i*60000}));
 rows[32].text="现在来看控制转移和syscall。";
 const reads=[], images=[];
 const call=async(method,path)=>{
  assert.equal(method,"GET");
  assert.ok(path.startsWith(prefix+"/"));
  const u=new URL("http://localhost"+path);
  if(u.pathname.endsWith("/transcript"))return rows;
  if(u.pathname.endsWith("/courseware"))return [doc,exam].map(path=>({path,name:path.split("/").at(-1),parse:{state:"done"}}));
  if(u.pathname.endsWith("/slides"))return [{idx:1,t_sec:0},{idx:2,t_sec:1900},{idx:3,t_sec:2500}];
  if(u.pathname.endsWith("/search"))return [
   {lecture_id:"other",file:doc,offset:100},{lecture_id:"L",file:"unknown",offset:0},
   {lecture_id:"L",file:doc,offset:53},{lecture_id:"L",file:doc,offset:99},{lecture_id:"L",file:exam,offset:3},
  ];
  if(u.pathname.endsWith("/displayed-page"))return {match:null};
  reads.push([u.searchParams.get("path"),u.searchParams.get("offset")]);
  return {text:"实际读取的原文"};
 };
 const evidence=await lectureEvidence(call,"L",async path=>{images.push(path);return "流程图";});
 assert.equal(images.length,0);
 const matches=await evidence.lookup("控制转移 syscall",{visual:true});
 assert.ok(matches.some(r=>r.kind==="transcript" && r.segments.some(s=>s.seconds===1920)));
 assert.deepEqual(reads,[[doc,"53"],[exam,"3"]]);
 assert.deepEqual(images,[prefix+"/slides/2/image"]);
 assert.ok(matches.some(r=>r.role==="past_exam"));
 assert.ok(matches.filter(r=>r.kind==="courseware").every(r=>r.text==="实际读取的原文"));
 assert.match(evidence.coverage(),/1 \/ 3/);
});
test("a revoked grant or cross-course note blocks the write",async()=>{
 const access=["library.read","workspace"];
 const mod={lessonStudy:{version:1,run:(_dir,_args,s)=>s.note("lesson-"+"1".repeat(32),"title","body")}};
 const plugins={enabled:()=>({mod,manifest:{permissions:access},coursive:{workspace:async()=>"unused"}}),state:{plugins:{"course-tutor":{granted:[]}}}};
 const call=async(method,path)=>{assert.equal(method,"GET");
  return path.startsWith("/api/lectures")?{lecture:{id:"L",course_id:"A"},course:{id:"A"}}:{course_id:"B",lecture_id:"L"};};
 const deps={plugins,call,memory:{prompt:async()=>""}};
 await assert.rejects(runLessonStudy(deps,"L",{action:"open"}),/允许/);
 plugins.state.plugins["course-tutor"].granted=access;
 await assert.rejects(runLessonStudy(deps,"L",{action:"open"}),/另一节/);
});
test("explicit preference scopes, corrections, forgetting and historic evidence",async t=>{
 const dir=await mkdtemp(join(tmpdir(),"learning-memory-"));t.after(()=>rm(dir,{recursive:true,force:true}));
 const m=new MemoryStore(dir);await m.init();
 const conv={course_id:"os",lastUser:"这门课我想先弄懂调度"};
 const remember=learningMemoryTool(m,conv);
 await remember.execute("1",{scope:"course",quote:conv.lastUser});
 await assert.rejects(remember.execute("2",{scope:"global",quote:conv.lastUser}),/范围/);
 conv.lastUser="以后都先举例";await remember.execute("3",{scope:"global",quote:conv.lastUser});
 assert.match(await m.prompt({pinnedOnly:true,courseId:"os"}),/调度/);
 assert.doesNotMatch(await m.prompt({pinnedOnly:true,courseId:"english"}),/调度/);
 assert.match(await m.prompt({pinnedOnly:true,courseId:"english"}),/先举例/);
 const path="courses/os.md";await m.write(path,"新目标",hashOf(await m.current(path)));
 const auto=memoryTools(m,{allowDelete:false,evidenceSince:1}).find(t=>t.name==="memory_append");
 await assert.rejects(auto.execute("4",{path,text:"旧判断"}),/手动更正/);
 await m.remove(path);await assert.rejects(auto.execute("5",{path,text:"旧判断"}),/手动更正/);
 assert.equal(await m.current(path),null);
});
