import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {CanvasCourseSources} from './canvas-course-sources.mjs';
import {CanvasRunner,CanvasToolScope,canvasContextKey,invalidQuotations} from './canvas-agent.mjs';
const text='😀原始材料';
const source={id:'file',path:'course/file.txt',lecture_id:'other-lecture',label:'file',offset:0};
const evidence={...source,availability:'readable',offset_unit:'rune',representation:'file_text',text,end:[...text].length,fingerprint:'verified'};
const context={workspace_id:'lecture',sources:[],object_ids:[]};
const courseScope=reply=>{const calls=[],course=new CanvasCourseSources(async(method,path,body)=>{calls.push({method,path,body});return path.endsWith('/search')?[source]:reply;},'/course');return {calls,course,scope:new CanvasToolScope(context,{course})};};
test('search grants no evidence; each read passage issues evidence with its own rune range',async()=>{
 const two='😀第一句原始材料。第二句也是原始材料。';
 const {calls,scope}=courseScope({...evidence,text:two,end:[...two].length,page:3});
 await assert.rejects(scope.execute('read_course_source',{source_id:'file'}),/未经/);
 const found=JSON.parse((await scope.execute('search_course_materials',{query:'原始'})).content[0].text);
 assert.equal(found.candidates.length,1);assert.equal(scope.evidence.size,0);
 await assert.rejects(scope.execute('read_course_source',{source_id:'file',offset:-1}),/范围/);
 const read=JSON.parse((await scope.execute('read_course_source',{source_id:'file'})).content[0].text);
 assert.deepEqual(read.passages.map(p=>p.evidence_id),['e1','e2']);
 const [a,b]=[...scope.evidence.values()];
 assert.equal(a.source_id,'course-file');assert.equal(a.kind,'exact_text');assert.equal(a.anchor.kind,'pdf');assert.equal(a.anchor.page,3);
 assert.deepEqual([a.anchor.start,a.anchor.end,b.anchor.start,b.anchor.end],[0,9,9,19]); // runes, not UTF-16 units
 assert.equal(a.anchor.original_path,source.path);assert.equal(a.anchor.source_lecture_id,'other-lecture');
 assert.equal(a.course_source.text,undefined);assert.deepEqual([b.course_source.offset,b.course_source.end],[9,19]);
 assert.equal(a.course_source.fingerprint,'verified');
 // Re-reading the same range reuses the IDs instead of minting new ones.
 await scope.execute('read_course_source',{source_id:'file'});assert.equal(scope.evidence.size,2);
 assert.equal(calls.at(-1).body.source.path,source.path);
});
test('malformed read ranges never become evidence',async()=>{
 const {scope}=courseScope({...evidence,end:999999});
 await scope.execute('search_course_materials',{query:'x'});
 await assert.rejects(scope.execute('read_course_source',{source_id:'file'}),/范围/);assert.equal(scope.evidence.size,0);
});
test('course tools are refused outside course scope',async()=>{
 await assert.rejects(new CanvasToolScope(context).execute('search_course_materials',{query:'x'}),/未启用课程范围/);
});
const bytes=Buffer.from('actual immutable image fixture'),hash=createHash('sha256').update(bytes).digest('hex');
async function finish(runner,id){for(let i=0;i<100;i++){const view=runner.status(id);if(['done','error','cancelled'].includes(view.state))return view;await new Promise(r=>setTimeout(r,2));}throw Error('timeout');}
for(const mode of ['image','unavailable'])test(`snapshot bytes reach vision, unavailable blocks model: ${mode}`,async()=>{
 let sawImage=false,modelCalls=0;
 const runner=new CanvasRunner({currentModel:async()=>({id:'fixture'}),runtime:{completeSimple:async(model,request)=>{modelCalls++;assert(request.messages.at(-1).content.some(c=>c.type==='image'&&c.data==='seen'));return {content:[{type:'text',text:'answer'}],stopReason:'stop'};}}},{call:async(method,path)=>path.endsWith('/context')?context:{id:'snapshot',sha256:hash},loadImage:async()=>({bytes,mimeType:'image/png'}),vision:async(pipeline,images)=>{assert.deepEqual(images[0].img.bytes,bytes);sawImage=true;return {mode,content:mode==='image'?[{type:'image',data:'seen',mimeType:'image/png'}]:[{type:'text',text:'unavailable'}]};}});
 const job=await runner.start({lecture_id:'lecture',question:'what is in this screenshot?',selection:{},attachments:[{id:'snapshot',sha256:hash}],context_scope:'course'});
 const done=await finish(runner,job.id);assert(sawImage);assert.equal(done.state,mode==='image'?'done':'error');assert.equal(modelCalls,mode==='image'?1:0);
});
test('course scope and workspace distinguish run inputs; a turn screenshot does not split history',()=>{
 const a=canvasContextKey(context,{context_scope:'course'});
 assert.notEqual(a,canvasContextKey(context,{context_scope:'selected'}));
 assert.notEqual(a,canvasContextKey({...context,workspace_id:'other'},{context_scope:'course'}));
 assert.equal(canvasContextKey(context,{attachments:[{id:'a',sha256:'a'}]}),canvasContextKey(context,{}));
});

for (const mode of ['image','described']) test(`persisted snapshot remains inspectable with no canvas bindings: ${mode}`, async () => {
 let round=0,visionCalls=0;
 const visual=mode==='image' ? {type:'image',data:'verified-image',mimeType:'image/png'} : {type:'text',text:'识图模型观察：圈出了条件 A'};
 const runner=new CanvasRunner({
  currentModel:async()=>({id:'fixture'}),
  runtime:{completeSimple:async(model,request)=>{
   round++;
   if(round===1){
    assert(request.messages.at(-1).content.some(c=>c.type===visual.type && (c.data===visual.data||c.text===visual.text)));
    return {content:[{type:'toolCall',id:'context',name:'get_canvas_context',arguments:{}}],stopReason:'toolUse'};
   }
   if(round===2){
    const result=JSON.parse(request.messages.at(-1).content[0].text);
    assert.deepEqual(result.sources,[]);
    assert.equal(result.context_scope,'course');
    assert.equal(result.attachments.length,1);
    assert.equal(result.attachments[0].already_in_message,true);
    assert.equal(result.attachments[0].availability,mode);
    return {content:[{type:'toolCall',id:'inspect',name:'inspect_canvas_region',arguments:{source_id:result.attachments[0].source_id}}],stopReason:'toolUse'};
   }
   const result=request.messages.at(-1);
   assert.equal(result.isError,false);
   assert.deepEqual(result.content[1],visual);
   return {content:[{type:'text',text:'Explain the circled condition'}],stopReason:'stop'};
  }},
 },{
  call:async(method,path)=>path.endsWith('/context')?context:{id:'snapshot',sha256:hash},
  loadImage:async()=>({bytes,mimeType:'image/png'}),
  vision:async()=>{visionCalls++;return {mode,content:[visual]};},
 });
 const job=await runner.start({lecture_id:'lecture',question:'Explain the selection',selection:{},attachments:[{id:'snapshot',sha256:hash}],context_scope:'course'});
 const result=await finish(runner,job.id);
 assert.equal(result.state,'done',result.error);
 assert.equal(round,3);
 assert.equal(visionCalls,1);
 assert.deepEqual(result.sources,[]); // A composite snapshot is not a fabricated object anchor.
 assert(result.trace.some(t=>t.tool==='inspect_canvas_region'&&t.status===mode));
 await runner.close();
});

test('course follow-up and reopened runner retain text plus verified previous pixels',async()=>{
 const turns=[];let calls=0,images=0;
 const pipeline={currentModel:async()=>({id:'fixture'}),runtime:{completeSimple:async(_model,request)=>{
  calls++;
  assert(request.messages.at(-1).content.some(c=>c.type==='image'&&c.data==='saved-pixels'));
  if(calls>1){assert(request.messages.some(m=>m.content.some(c=>c.text==='first explanation')));}
  return {content:[{type:'text',text:calls===1?'first explanation':'follow-up explanation'}],stopReason:'stop'};
 }}};
 const options={call:async(_method,path)=>path.endsWith('/context')?context:path.includes('/conversations/')?{turns}:{id:'snapshot',sha256:hash},loadImage:async()=>({bytes,mimeType:'image/png'}),vision:async()=>{images++;return {mode:'image',content:[{type:'image',mimeType:'image/png',data:'saved-pixels'}]};}};
 const request={lecture_id:'lecture',conversation_id:'same',question:'first question',selection:{},context_scope:'course',attachments:[{id:'snapshot',sha256:hash}]};
 let runner=new CanvasRunner(pipeline,options);
 let done=await finish(runner,(await runner.start(request)).id);
 assert.equal(done.state,'done');turns.push({...done,question:request.question,context_scope:'course',attachments:request.attachments});
 for(const reopened of [false,true]){
  if(reopened)runner=new CanvasRunner(pipeline,options);
  done=await finish(runner,(await runner.start({...request,question:'Explain that again',attachments:[]})).id);
  assert.equal(done.state,'done');
  turns.push({...done,question:'Explain that again',context_scope:'course',attachments:[]});
 }
 assert.equal(images,3);assert.equal(calls,3);
});

test('history obeys retry cutoff, scope boundary, and does not turn old citations into new evidence',async()=>{
 const turns=[
  {id:'old',state:'done',context_scope:'course',question:'revoked',answer:'revoked',attachments:[{id:'revoked',sha256:hash}]},
  {id:'boundary',state:'done',context_scope:'selected',question:'selected',answer:'selected'},
  {id:'allowed',state:'done',context_scope:'course',question:'allowed',answer:'old cite [1](coursive://citation/e1) [[e:e2]]'},
  {id:'retry',state:'error',context_scope:'course',question:'retry',answer:''},
  {id:'future',state:'done',context_scope:'course',question:'future',answer:'future',attachments:[{id:'future',sha256:hash}]},
 ];
 const runner=new CanvasRunner({currentModel:async()=>({id:'fixture'}),runtime:{completeSimple:async(_model,request)=>{
  const text=JSON.stringify(request.messages);assert.match(text,/allowed/);assert.doesNotMatch(text,/revoked|future|selected|coursive:\/\/citation|\[\[e:/);
  return {content:[{type:'text',text:'not re-read [1](coursive://citation/e1)'}],stopReason:'stop'};
 }}},{call:async(_method,path)=>path.endsWith('/context')?context:{turns},loadImage:async()=>{throw Error('must not load inaccessible history');}});
 const done=await finish(runner,(await runner.start({lecture_id:'lecture',conversation_id:'same',turn_id:'retry',question:'retry',selection:{},context_scope:'course',attachments:[]})).id);
 assert.equal(done.state,'done');assert.deepEqual(done.evidence,[]);assert.equal(done.answer,'not re-read');
});

test('a new screenshot leaves older saved screenshots discoverable for explicit rereading',async()=>{
 let round=0;const loaded=[];
 const runner=new CanvasRunner({currentModel:async()=>({id:'fixture'}),runtime:{completeSimple:async(_model,request)=>{
  round++;
  if(round===1)return {content:[{type:'toolCall',id:'context',name:'get_canvas_context',arguments:{}}]};
  if(round===2){
   const c=JSON.parse(request.messages.at(-1).content[0].text);
   assert.equal(c.attachments.length,2);
   assert.equal(c.attachments.find(a=>a.source_id==='attachment-old').already_in_message,false);
   return {content:[{type:'toolCall',id:'read',name:'inspect_canvas_region',arguments:{source_id:'attachment-old'}}]};
  }
  assert.equal(request.messages.at(-1).isError,false);
  assert(request.messages.at(-1).content.some(c=>c.type==='image'));
  return {content:[{type:'text',text:'old image reread'}],stopReason:'stop'};
 }}},{call:async(_method,path)=>path.endsWith('/context')?context:path.includes('/conversations/')?{turns:[{id:'old-turn',question:'old question',answer:'old answer',state:'done',context_scope:'course',attachments:[{id:'old',sha256:hash}]}]}:{id:path.split('/').at(-1),sha256:hash},
 loadImage:async(path)=>{loaded.push(path);return {bytes,mimeType:'image/png'};},vision:async()=>({mode:'image',content:[{type:'image',data:'pixels',mimeType:'image/png'}]})});
 const done=await finish(runner,(await runner.start({lecture_id:'lecture',conversation_id:'same',selection:{},question:'compare to old',context_scope:'course',attachments:[{id:'new',sha256:hash}]})).id);
 assert.equal(done.state,'done');assert.equal(loaded.length,2);assert.match(loaded[0],/new\/image$/);assert.match(loaded[1],/old\/image$/);
});

test('transcript evidence label and anchor cover the verified interval including zero', async () => {
 const {scope}=courseScope({...evidence,time_end_ms:12345,video_path:'course/v.mp4'});
 await scope.execute('search_course_materials',{query:'x'});
 await scope.execute('read_course_source',{source_id:'file'});
 const e=scope.evidence.get('e1');
 assert.match(e.label,/0–13秒/);assert.equal(e.kind,'timestamped_transcript');
 assert.deepEqual([e.anchor.kind,e.anchor.time_start_ms,e.anchor.time_end_ms],['video',0,12345]);
 assert.equal(e.course_source.video_path,'course/v.mp4');
});

test('a copied historical quotation cannot attach to a new source with the same evidence ID',async()=>{
 const passage='这段原文介绍银行家算法的安全性检查。';
 const actual={...evidence,text:passage,end:[...passage].length};
 let round=0;
 const runner=new CanvasRunner({currentModel:async()=>({id:'fixture'}),runtime:{completeSimple:async(_model,request)=>{
  round++;
  if(round===1)return {content:[{type:'toolCall',id:'s',name:'search_course_materials',arguments:{query:'banker'}}]};
  if(round===2)return {content:[{type:'toolCall',id:'r',name:'read_course_source',arguments:{source_id:'file'}}]};
  if(round===3)return {content:[{type:'text',text:'“这段完全不同的旧引文介绍信号量与进程同步。”[[e:e1]]'}],stopReason:'stop'};
  assert.match(request.messages.at(-1).content[0].text,/系统引用核验/);
  return {content:[{type:'text',text:`“${passage}”[[e:e1]]`}],stopReason:'stop'};
 }}},{call:async(_method,path)=>path.endsWith('/context')?context:path.endsWith('/search')?[source]:actual});
 const done=await finish(runner,(await runner.start({lecture_id:'lecture',selection:{},question:'quote',context_scope:'course'})).id);
 assert.equal(done.state,'done');assert.equal(round,4);assert.match(done.answer,/银行家算法/);assert.equal(done.evidence.length,1);assert.equal(done.evidence[0].source_id,'course-file');
 assert(done.trace.some(t=>t.tool==='validate_citations'&&t.status==='retry'));
});

test('literal quote checks ignore punctuation and code examples without treating topic labels as quotes',async()=>{
 const passage='我们来看一道银行家算法的练习题啊，这个题非常重要。';
 const actual={...evidence,text:passage,end:[...passage].length};
 const {scope}=courseScope(actual);
 await scope.execute('search_course_materials',{query:'x'});
 await scope.execute('read_course_source',{source_id:'file'});
 assert.deepEqual(invalidQuotations('“我们来看一道银行家算法的练习题啊。”[[e:e1]]',scope),[]);
 assert.deepEqual(invalidQuotations('“这是不存在于该来源的另一段直接引文。”[[e:e1]]',scope),['e1']);
 assert.deepEqual(invalidQuotations('example `“这是不存在于该来源的另一段直接引文。”[[e:e1]]`',scope),[]);
 assert.deepEqual(invalidQuotations('“下面这个题目的名字是银行家算法的例子”，见如下解释。 [[e:e1]]',scope),[]);
});

test('a selected-scope follow-up replays the turn that attached a screenshot and sees its pixels',async()=>{
 const turns=[{id:'t1',state:'done',context_scope:'selected',question:'what is this',answer:'first look',attachments:[{id:'snapshot',sha256:hash}]}];
 let seen;
 const runner=new CanvasRunner({currentModel:async()=>({id:'fixture'}),runtime:{completeSimple:async(_model,request)=>{seen=request;return {content:[{type:'text',text:'again'}],stopReason:'stop'};}}},
  {call:async(_m,path)=>path.endsWith('/context')?context:path.includes('/conversations/')?{turns}:{id:'snapshot',sha256:hash},loadImage:async()=>({bytes,mimeType:'image/png'}),vision:async()=>({mode:'image',content:[{type:'image',data:'pixels',mimeType:'image/png'}]})});
 turns[0].context_key=canvasContextKey(context,{});
 const done=await finish(runner,(await runner.start({lecture_id:'lecture',conversation_id:'c',question:'explain again',selection:{}})).id);
 assert.equal(done.state,'done',done.error);
 assert(seen.messages.some(m=>m.content.some(c=>c.text==='first look')));
 assert(seen.messages.at(-2).content.some(c=>c.type==='image'&&c.data==='pixels'));
 assert(!seen.tools.some(t=>t.name==='search_course_materials'));
});

test('quotes from bound material are checked against the text that was read',async()=>{
 const doc={workspace_id:'lecture',object_ids:['o'],sources:[{id:'o',label:'讲义',kind:'text',availability:'available',text:'调度器在时钟中断时检查就绪队列。',anchor:{id:'o',kind:'text',start:0,end:16}}]};
 const scope=new CanvasToolScope(doc);
 await scope.execute('read_document_selection',{source_id:'o'});
 assert.deepEqual(invalidQuotations('“调度器在时钟中断时检查就绪队列”[[e:e1]]',scope),[]);
 assert.deepEqual(invalidQuotations('“调度器每秒都会重新排序全部进程”[[e:e1]]',scope),['e1']);
});
