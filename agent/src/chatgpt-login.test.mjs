import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, stat } from "node:fs/promises";
import { createServer } from "node:net";
import { ModelRuntime } from "@earendil-works/pi-coding-agent";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { chatGPTLogin } from "./chatgpt-login.mjs";

const deferred = () => { let resolve; const promise = new Promise(r => { resolve = r; }); return { promise, resolve }; };
async function fixture(t, login) {
 const directory = await mkdtemp(join(tmpdir(), "coursive-login-test-"));
 t.after(() => rm(directory, { recursive: true, force: true }));
 const selected = [];
 const pipeline = { prefs: {model:"unavailable"}, runtime: {login, getAvailable: async () => [{provider:"other",id:"first"},{provider:"openai",id:"catalog-gpt"}]}, setPrefs: async p => selected.push(p) };
 return {flow:chatGPTLogin(pipeline,directory),directory,selected};
}
async function done(flow, status) {
 for (let n=0; n<50 && flow.status().status!==status; n++) await new Promise(r=>setTimeout(r,5));
 assert.equal(flow.status().status,status);
}

test("concurrent starts share a login, keep device identity private and use the model catalog", async t => {
 const finish=deferred(); let calls=0, device;
 const {flow,directory,selected}=await fixture(t,async (provider,type,interaction,options) => {
  calls++; assert.equal(provider,"openai"); assert.equal(type,"oauth"); device=options.getDeviceId();
  interaction.notify({type:"auth_url",url:"https://auth.openai.com/authorize?state=synthetic"});
  await finish.promise;
 });
 const [a,b]=await Promise.all([flow.start(),flow.start()]);
 assert.deepEqual(a,b); assert.equal(calls,1); assert.equal(flow.status().status,"waiting");
 const file=join(directory,"oauth-device-id"); assert.equal(await readFile(file,"utf8"),device);
 assert.equal((await stat(file)).mode & 0o777,0o600);
 finish.resolve(); await done(flow,"done");
 assert.deepEqual(selected,[{provider:"openai",model:"catalog-gpt"}]);
 assert.deepEqual(flow.status(),{status:"done"});
});

test("an untrusted authorization URL fails before it reaches the app",async t=>{
 const {flow}=await fixture(t,async (_p,_k,i)=>i.notify({type:"auth_url",url:"https://untrusted.example/authorize"}));
 await assert.rejects(flow.start(),/登录未完成/); await done(flow,"error");
 assert.equal(flow.status().phase,"authorization");
});

test("a token exchange failure clears active state and exposes no provider detail",async t=>{
 const finish=deferred();
 const {flow}=await fixture(t,async (_p,_k,i)=>{
  i.notify({type:"auth_url",url:"https://auth.openai.com/authorize"}); await finish.promise;
  i.notify({type:"progress",message:"Exchanging authorization code for tokens..."});
  throw new Error("sensitive provider response");
 });
 await flow.start(); finish.resolve(); await done(flow,"error");
 assert.equal(flow.status().phase,"token_exchange");
 assert.doesNotMatch(JSON.stringify(flow.status()),/sensitive/);
});

test("a completed SDK login without a browser URL resolves instead of hanging",async t=>{
 const {flow}=await fixture(t,async()=>{});
 assert.deepEqual(await flow.start(),{status:"done"});
});

test("a busy loopback port fails before the bundled SDK opens a browser", {timeout: 8000}, async t => {
 const directory = await mkdtemp(join(tmpdir(), "coursive-sdk-login-"));
 t.after(() => rm(directory, { recursive: true, force: true }));
 const runtime = await ModelRuntime.create({
  authPath: join(directory, "auth.json"), modelsPath: join(directory, "models.json"),
  modelsStorePath: join(directory, "store.json"), allowModelNetwork: false, refreshOnCreate: false,
 });
 const listener = createServer();
 try {
  await new Promise((resolve, reject) => listener.once("error", reject).listen(1455, "127.0.0.1", resolve));
  t.after(() => new Promise(resolve => listener.close(resolve)));
 } catch (error) {
  if (error.code !== "EADDRINUSE") throw error;
 }
 // The standalone SDK throws on bind failure. Its bundled OAuth provider
 // reports the same failure through notify() before offering manual entry.
 const chunk = new URL("./chunks/openai-chatgpt.js", import.meta.resolve("@earendil-works/pi-coding-agent/rpc-entry"));
 const {openaiChatGPTOAuth} = await import(chunk);
 const bundled = {login: (_provider, _type, interaction, options) => openaiChatGPTOAuth.login(interaction, options)};
 for (const sdk of [runtime, bundled]) {
  const flow = chatGPTLogin({runtime: sdk}, directory);
  await assert.rejects(flow.start(), /1455/);
  assert.equal(flow.status().phase, "callback_busy");
 }
});
