// A course's file inventory and the lecture evidence reader are shared with
// the library and teaching plugin. A conversation never supplies a filesystem path.
import { lectureEvidence, transcriptSources } from "./lesson-study.mjs";
import { transcriptIndex } from "./teaching-evidence.mjs";

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
