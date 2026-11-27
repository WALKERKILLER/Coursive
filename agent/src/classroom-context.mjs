// A course's file inventory and the lecture evidence reader are shared with
// the library and teaching plugin. A conversation never supplies a filesystem path.
import { lectureEvidence, transcriptSources } from "./lesson-study.mjs";
import { digest, transcriptIndex } from "./teaching-evidence.mjs";

export class ClassroomContext {
  constructor(call, lectureId, verify, readSlide) {
    Object.assign(this, { call, lectureId, verify, readSlide });
  }

  async load() {
    const { course, lecture } = await this.call("GET", "/api/lectures/" + encodeURIComponent(this.lectureId));
    if (!course?.id || lecture?.course_id !== course.id) throw Error("课程范围无效");
    const [roster, catalog] = await Promise.all([
      this.call("GET", "/api/courses/" + encodeURIComponent(course.id) + "/lectures"),
      this.call("GET", "/api/library/catalog?course_id=" + encodeURIComponent(course.id)),
    ]);
    return { lectures: roster.lectures, files: catalog.filter(f => f.course_id === course.id) };
  }

  async inventory(id = this.lectureId) {
    this.pending ??= this.load().catch(e => { this.pending = null; throw e; });
    const { lectures, files } = await this.pending;
    if (!lectures.some(l => l.id === id)) throw Error("节次不属于当前课程");
    const material = f => ({ name: f.name, path: f.path, type: f.file_type, ...(f.note_id && { note_id: f.note_id }) });
    return {
      lecture_id: id,
      lectures: lectures.map(l => ({ id: l.id, date: l.date, title: l.title, assets: l.assets, transcript_lines: l.segments })),
      materials: files.filter(f => f.lecture_id === id && !["slides","canvas"].includes(f.category) &&
        ["mp4","mkv","mov","webm","srt","vtt","ppt","pptx","md","pdf"].includes(f.file_type)).map(material),
      shared_slides: files.filter(f => !f.lecture_id && ["ppt","pptx"].includes(f.file_type)).map(material),
    };
  }

  async linked(markdown, excerpt) {
    const refs = adjacentLessonRefs(markdown, excerpt);
    if (!refs.length) return [];
    const { lectures } = await this.inventory();
    const out = [];
    for (const { label,link } of refs) {
      const url=new URL(link), params=url.searchParams;
      const [,id,idx]=url.pathname.split("/");
      if (!lectures.some(l=>l.id===id) || params.get("v")!=="1") continue;
      const base="/api/lectures/"+encodeURIComponent(id);
      if (url.host==="lecture") {
        const from=Number(params.get("start_ms")),to=Number(params.get("end_ms")),quote=params.get("q");
        if (!quote || !Number.isSafeInteger(from) || !Number.isSafeInteger(to) || from<0 || to<=from || to-from>300000) continue;
        const segs=await this.call("GET",base+"/transcript?from_ms="+from+"&to_ms="+to);
        if (!segs.length || segs[0].start_ms!==from || !segs.filter(s=>s.start_ms<to).map(s=>s.text.trim()).join("\n").includes(quote)) continue;
        out.push({ id:"ref-"+digest(link).slice(0,24),lectureId:id,kind:"transcript",fingerprint:digest(quote),verifiedQuote:quote,label,link });
      } else if (url.host==="slide" && /^\d+$/.test(idx) && /^[a-f0-9]{64}$/.test(params.get("image_rev") ?? "")) {
        const slides=await this.call("GET",base+"/slides?local=1");
        if (!slides.some(s=>s.idx===Number(idx))) continue;
        // The viewer compares the image fingerprint on open. The label describes
        // the handout's binding; no uninspected pixel is reported as OCR.
        out.push({id:"ref-"+digest(link).slice(0,24),lectureId:id,kind:"slide",fingerprint:params.get("image_rev"),verifiedQuote:"",label,link});
      }
    }
    return out;
  }

  async find(query, question, id, visual, signal) {
    const inventory = await this.inventory(id);
    if (id) {
      const reader = await lectureEvidence(this.call, id, visual ? this.readSlide : null,
        (q, items) => this.verify(q, items, signal), { localSlides: true });
      return reader.lookup(query, { visual, question });
    }
    if (!this.courseIndex) {
      const windows = [];
      for (const lecture of inventory.lectures) {
        signal?.throwIfAborted();
        if (!lecture.transcript_lines) continue;
        const segments = await this.call("GET", "/api/lectures/" + encodeURIComponent(lecture.id) + "/transcript");
        windows.push(...transcriptSources(lecture.id, segments));
      }
      this.courseIndex = transcriptIndex(windows);
    }
    return this.verify(question, this.courseIndex(query).map(hit => hit.item), signal);
  }
}

// SelectionArea returns rendered text (often several list items). Look only
// at a uniquely matched block and its immediately following citation row.
export function adjacentLessonRefs(markdown, excerpt) {
  const flatten = text => text.replace(/^[ \t]*(?:[-*+]|\d+\.)\s+/gm,"").replace(/[\s*_`]/gu,"");
  const needle = flatten(excerpt);
  if (needle.length < 4) return [];
  const blocks = markdown.split(/\n\s*\n/u);
  const matches = blocks.flatMap((b,i) => flatten(b).includes(needle) ? [i] : []);
  if (matches.length !== 1) return [];
  const line = blocks[matches[0]+1]?.trim() ?? "";
  if (!line.startsWith("[") || line.length > 8000) return [];
  return [...line.matchAll(/\[([^\]\n]{1,140})\]\((coursive:\/\/[^\s)]+)\)/gu)].slice(0,6)
    .map(m => ({label:m[1], link:m[2]}));
}
