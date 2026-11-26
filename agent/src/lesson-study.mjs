import { createHash } from "node:crypto";
import { readLessonSlide } from "./vision.mjs";
import { supportedClaims, verifiedLookup } from "./teaching-lookup.mjs";
import { transcriptIndex } from "./teaching-evidence.mjs";
import { entryFrom } from "./usage.mjs";

const hash = text => createHash("sha256").update(text).digest("hex");
const enc = encodeURIComponent;
const idPattern = /^[A-Za-z0-9_-]{1,100}$/;
const limit = 24000;

function source(lectureId, kind, text, label, link, position, segments, role) {
  const fingerprint = hash(text);
  const id = "e_" + hash(JSON.stringify([lectureId, kind, link, position, fingerprint])).slice(0, 24);
  return {id, kind, text, label, link, position, segments, role, fingerprint, lectureId};
}

// Sample throughout a long reading rather than silently presenting its first page as the lesson.
function sample(items, count) {
  if (items.length <= count) return items;
  return Array.from({length:count}, (_,i) => items[Math.round(i*(items.length-1)/(count-1))]);
}

function transcriptSources(lectureId, segments) {
  const windows = [];
  for (const row of segments ?? []) {
    if (!row.text?.trim() || !Number.isFinite(row.start_ms)) continue;
    let w = windows.at(-1);
    if (!w || w.text.length + row.text.length > 720) {
      w={pos:Math.floor(row.start_ms/1000), text:"", rows:[]}; windows.push(w);
    }
    w.rows.push({at:w.text.length + (w.text?1:0), seconds:Math.floor(row.start_ms/1000),
      ...(Number.isFinite(row.end_ms) && row.end_ms>row.start_ms ? {startMs:row.start_ms,endMs:row.end_ms} : {})});
    w.text += (w.text?"\n":"") + row.text.trim();
  }
  return windows.map(w => source(lectureId, "transcript", w.text.slice(0,850), "课堂录音 · "+w.pos+" 秒",
    "coursive://lecture/"+enc(lectureId)+"?t="+w.pos, w.pos, w.rows));
}

export async function lectureMaterials(call, lectureId, readSlide) {
  const base = "/api/lectures/" + enc(lectureId);
  const [segments, files] = await Promise.all([call("GET",base+"/transcript"), call("GET",base+"/courseware")]);
  const windows = transcriptSources(lectureId, segments);
  const chosen = sample(windows,20);
  const sources = [...chosen], gaps = [];
  const add = (...args) => sources.push(source(lectureId, ...args));
  if (!windows.length) gaps.push("本节缺少可读转写");
  else if (chosen.length<windows.length) gaps.push("转写按课堂顺序抽取了 "+chosen.length+" / "+windows.length+" 段");
  const parsed = (files ?? []).filter(f=>f.parse?.state==="done");
  const budgeted = sample(parsed,6);
  if (budgeted.length < (files ?? []).length) gaps.push("读取了 "+budgeted.length+" / "+files.length+" 份课件");
  if (!(files??[]).length) gaps.push("本节没有关联课件");
  for (const f of budgeted) {
    const at = offset => call("GET",base+"/courseware/text?path="+enc(f.path)+"&offset="+offset+"&max=800");
    const first = await at(0);
    if (!first.text?.trim()) continue;
    const offsets = [...new Set([0,Math.max(0,Math.floor((first.total-800)/2)), Math.max(0,first.total-800)])];
    if (first.total > 2400) gaps.push(f.name+" 已抽取开头、中段和结尾");
    for (const offset of offsets) {
      const part = offset===0 ? first : await at(offset);
      const text = part.text?.trim();
      if (text) add("courseware",text,f.name,"coursive://courseware/"+enc(lectureId)+"?file="+enc(f.name)+"&q="+enc([...text].slice(0,28).join("")),offset);
    }
  }
  if (readSlide) {
    const slides = await call("GET",base+"/slides");
    const selected = sample((slides ?? []).filter(s=>Number.isInteger(s.idx)),3);
    let read = 0;
    const signal = AbortSignal.timeout(45000);
    for (const slide of selected) {
      try {
        const text = await readSlide(base+"/slides/"+slide.idx+"/image",signal);
        if (!text?.trim()) break;
        add("slide",text.trim().slice(0,1600),"课件截图 "+slide.idx+" · 图像转写",
          "coursive://slide/"+enc(lectureId)+"/"+slide.idx,slide.idx);
        read++;
      } catch { if (signal.aborted) break; }
    }
    if (slides?.length) gaps.push("课件截图已识读 "+read+" / "+slides.length+" 张，截图文字可点回原图核对");
  }
  return {sources, coverage:gaps.length ? "资料范围 · "+gaps.join("。")+"。" : ""};
}

const readRole = name => {
  if (/答案|答题纸|answer[ _-]*key/iu.test(name)) return "supplement";
  if (/真题|往年卷|往年试卷|历年试卷|past[ _-]*(exam|paper)/iu.test(name)) return "past_exam";
  return /教材|参考书|textbook|reading/iu.test(name) ? "supplement" : "courseware";
};

// All input paths are bound to the lecture before the model can request a lookup.
export async function lectureEvidence(call, lectureId, readSlide, verify, {localSlides=false}={}) {
  const base = "/api/lectures/" + enc(lectureId);
  const [segments, files, slides] = await Promise.all([
    call("GET", base+"/transcript"), call("GET", base+"/courseware"),
    call("GET", base+(localSlides?"/slides?local=1":"/slides")).catch(() => []),
  ]);
  const windows = transcriptSources(lectureId, segments);
  let searchTranscript;
  const parsed = new Map((files ?? []).filter(f => f.parse?.state === "done").map(f => [f.path,f]));
  const pictures = (slides ?? []).filter(s => Number.isInteger(s.idx))
    .sort((a,b) => a.t_sec-b.t_sec);
  const inspected = new Map();
  let lookups = 0, figures = 0, failed = 0, signal;
  const catalog = {
    timeline: sample(windows, 14).map(w => ({seconds:w.position, preview:w.text.slice(0,380)})),
    files: (files ?? []).slice(0, 24).map(f => ({name:f.name, role:readRole(f.name), readable:parsed.has(f.path)})),
    screenshots:pictures.length,
  };
  async function see(slide) {
    if (inspected.has(slide.idx)) return inspected.get(slide.idx);
    signal ??= AbortSignal.timeout(60000);
    if (!readSlide || figures >= 6 || signal.aborted) return null;
    figures++;
    try {
      const image = await readSlide(base+"/slides/"+slide.idx+"/image",signal);
      const text = (typeof image==="string" ? image : image?.text)?.trim().slice(0, 1100);
      const item = text ? source(lectureId,"slide",text,"课件截图 "+slide.idx,"coursive://slide/"+enc(lectureId)+"/"+slide.idx,slide.idx) : null;
      if (item && image?.imageHash) item.imageHash=image.imageHash;
      inspected.set(slide.idx,item);
      return item;
    } catch {
      failed++;
      inspected.set(slide.idx,null);
      return null;
    }
  }
  async function readFileExcerpt(file, offset) {
    try {
      const part = await call("GET", base+"/courseware/text?path="+enc(file.path)+"&offset="+offset+"&max=750");
      const text = part.text?.trim();
      if (text) return source(lectureId,"courseware",text,file.name,
        "coursive://courseware/"+enc(lectureId)+"?file="+enc(file.path),offset,undefined,readRole(file.name));
    } catch { failed++; }
    return null;
  }
  async function lookup(query, {visual = false, seconds, question} = {}) {
    if (typeof query !== "string" || !query.trim() || query.length > 140 || ++lookups > 10) return [];
    searchTranscript ??= transcriptIndex(windows);
    const ranked = searchTranscript(query);
    const matches = ranked.map(hit=>({...hit.item, position:hit.seconds, link:"coursive://lecture/"+enc(lectureId)+"?t="+hit.seconds}));
    const near = Number.isFinite(seconds) ? windows.findLast(w => w.position <= seconds) : null;
    if (near && !matches.some(x=>x.id===near.id)) matches.push(near);
    const hits = parsed.size
      ? await call("GET", base+"/courseware/search?limit=8&q="+enc(query)).catch(() => { failed++; return []; })
      : [];
    const reading = (hits ?? []).filter(h => h.lecture_id === lectureId && parsed.has(h.file) &&
      Number.isInteger(h.offset) && h.offset >= 0);
    const distinctFiles = [...new Set(reading.map(h => h.file))].slice(0, 3);
    const excerpts = await Promise.all(distinctFiles.map(path =>
      readFileExcerpt(parsed.get(path),reading.find(hit=>hit.file===path).offset)));
    const slidesForTopic = [];
    if (visual) {
      const anchors = [...new Set([
        ...ranked.map(hit=>hit.seconds).filter(Number.isFinite).slice(0,2),
        ...(Number.isFinite(seconds) ? [seconds] : []),
      ])];
      for (const second of anchors) {
        const active = pictures.findLast(s => s.t_sec <= second);
        if (active && second - active.t_sec <= 150) slidesForTopic.push(active);
      }
      if (!slidesForTopic.length) slidesForTopic.push(...sample(pictures,3));
    }
    for (const picture of new Set(slidesForTopic)) {
      const slide = await see(picture);
      if (!slide) continue;
      excerpts.push(slide);
      if (!parsed.size) continue;
      const result = await call("GET",base+"/courseware/displayed-page?visible="+enc(slide.text)).catch(()=>null);
      const page = result?.match;
      if (!parsed.has(page?.file) || !Number.isInteger(page.page) || page.page<1 ||
          !page.text || !/^[a-f0-9]{64}$/.test(page.revision)) continue;
      const url = new URL("coursive://courseware/"+enc(lectureId));
      url.searchParams.set("file",page.file);
      url.searchParams.set("page",String(page.page));
      url.searchParams.set("slide",String(picture.idx));
      url.searchParams.set("index_rev",page.revision);
      excerpts.push(source(lectureId,"courseware",page.text,page.name+" · 第"+page.page+"页",url.href,page.page,undefined,"displayed_page"));
    }
    const candidates=[...matches,...excerpts.filter(Boolean)];
    return verify ? verify(question || query,candidates) : candidates;
  }
  return {
    catalog,
    lookup,
    async overviewSources() {
      if (windows.length) return sample(windows, 8);
      const selected = [...parsed.values()].filter(f=>readRole(f.name) !== "past_exam").slice(0, 3);
      return (await Promise.all(selected.map(file=>readFileExcerpt(file,0)))).filter(Boolean);
    },
    async readCourseware(limit = 2) {
      return (await Promise.all([...parsed.values()].slice(0, limit).map((file) => readFileExcerpt(file, 0)))).filter(Boolean);
    },
    coverage: () => [
      !windows.length && "本节缺少可读转写",
      parsed.size < (files ?? []).length && "部分课件尚未解析",
      pictures.length && "截图按需识读 "+[...inspected.values()].filter(Boolean).length+" / "+pictures.length+" 张",
      failed && "部分资料读取失败",
    ].filter(Boolean).join("。"),
  };
}

export async function runLessonStudy({plugins,memory,pipeline,call},lectureId,input) {
  if (!idPattern.test(lectureId) || !["open","status","attempt","position","answer","revise","hint","skip"].includes(input?.action))
    throw Error("无效的课程学习请求。");
  if (input.action==="answer" && (!idPattern.test(input.submissionId??"") || !idPattern.test(input.questionId??"") || typeof input.answer!=="string"))
    throw Error("请从当前题目提交答案。");
  const {course,lecture} = await call("GET","/api/lectures/"+enc(lectureId));
  if (!course?.id || lecture?.id !== lectureId || lecture.course_id !== course.id) throw Error("无法确认节次所属课程。");
  function plugin() {
    const rec=plugins.enabled("course-tutor");
    if (rec.mod?.lessonStudy?.version!==1 || !["library.read","workspace"].every(key=>rec.manifest.permissions.includes(key) && plugins.state.plugins["course-tutor"]?.granted?.includes(key))) throw Error("请在插件页启用支持讲义的课程伴学，并允许读取资料和工作目录。");
    return rec;
  }
  const rec=plugin();
  const dir=await rec.coursive.workspace("course-"+hash(course.id).slice(0,48));
  const needsMemory=["open","revise"].includes(input.action);
  const deadline = AbortSignal.timeout(330000);
  return rec.mod.lessonStudy.run(dir,{...input,course:course.id,lecture:lectureId,title:lecture.title||course.title,
    memory: needsMemory ? await memory.prompt({pinnedOnly:true,courseId:course.id}) : ""},{
    skillTracking:true,
    supportedClaims:claims=>{plugin(); return supportedClaims(pipeline,claims,deadline);},
    materials:()=>lectureMaterials(call,lectureId,(path,signal)=>readLessonSlide(pipeline,path,signal)),
    evidence:()=>{
      plugin();
      return lectureEvidence((...args)=>{ plugin(); return call(...args); }, lectureId,
        (path,signal)=>{ plugin(); return readLessonSlide(pipeline,path,signal,true); },
        (query,items)=>{plugin();return verifiedLookup(pipeline,query,items,deadline);});
    },
    model:async (systemPrompt,payload)=>{
      plugin();
      const model=await pipeline.currentModel();
      if (!model) throw Error("请先在小 C 里选择可用模型。");
      const result=await pipeline.runtime.completeSimple(model,{systemPrompt,messages:[{role:"user",content:JSON.stringify(payload),timestamp:Date.now()}]},
        {signal:AbortSignal.any([deadline,AbortSignal.timeout(150000)]),maxTokens:8000});
      pipeline.usage?.record(entryFrom(result,{purpose:"lesson"}));
      if (["error","aborted","length"].includes(result.stopReason)) throw Error("模型未完成本次备课或评阅，请重试。");
      const raw=result.content.filter(c=>c.type==="text").map(c=>c.text).join("").trim();
      if (raw.length>limit) throw Error("讲义过长，请重试。");
      plugin();
      return JSON.parse(raw.replace(/^\x60\x60\x60(?:json)?\s*|\s*\x60\x60\x60$/gi,""));
    },
    note:async (id,title,body)=>{
      plugin();
      if (!/^lesson-[a-f0-9]{32}$/.test(id) || body.length>limit || typeof title!=="string" || !title.trim() || title.length>240) throw Error("讲义草稿无效。");
      const path="/api/notes/"+id;
      const existing=async ()=>{
        const n=await call("GET",path);
        if(n.lecture_id!==lectureId || n.course_id!==course.id) throw Error("已有笔记属于另一节课。");
        return n;
      };
      try { return await existing(); } catch(e) { if(e.status!==404) throw e; }
      try { return await call("PUT","/api/notes",{id, title, body, create_only:true, course_id:course.id, lecture_id:lectureId, source:"agent"}); }
      catch(e){ if(e.status===409) return existing(); throw e; }
    },
  });
}
