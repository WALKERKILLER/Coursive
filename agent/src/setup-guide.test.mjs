import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';

test('setup guide reads live tool status and preserves unknown older-daemon fields', async () => {
 let health = {ffmpeg: true, media_tools: {ffmpeg: true, ffprobe: false, pdftotext: false}, library: '/private/library'};
 const server = createServer((req, res) => {
  assert.equal(req.url, '/api/health');
  res.setHeader('Content-Type', 'application/json'); res.end(JSON.stringify(health));
 });
 await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
 const prior = process.env.COURSIVE_URL;
 process.env.COURSIVE_URL = `http://127.0.0.1:${server.address().port}`;
 try {
  const {createTools} = await import('./tools.mjs');
  const guide = createTools({}).find(tool => tool.name === 'app_guide');
  let result = await guide.execute('setup', {topic: 'setup'});
  assert.match(result.content[0].text, /"ffprobe": false/);
  assert.doesNotMatch(result.content[0].text, /private\/library/);
  health = {ffmpeg: false};
  result = await guide.execute('setup', {topic: 'setup'});
  assert.match(result.content[0].text, /"media_tools": null/);
 } finally {
  if (prior === undefined) delete process.env.COURSIVE_URL; else process.env.COURSIVE_URL = prior;
  server.closeAllConnections(); await new Promise(resolve => server.close(resolve));
 }
});
