(function(){
'use strict';
var FROM_HOST='lg-replay-host';
var FROM_FRAME='lg-replay-frame';
var SCHEMA=2;
var DEFAULT_ASSET_PREFIXES=['/lg/assets/'];
var DEFAULT_NOTICE='Not in this recording: {request}. The replay holds the answers of the tour and of the rows its pages show.';
var DEFAULT_NOTICE_CLASS='error-banner';
var NOTICE_REQUEST='{request}';
var CHUNK_FILE=/^[a-z0-9][a-z0-9-]*\.json$/;
var CHUNK_WAIT_MS=30000;
var STREAM_INTERVAL_MS=5000;
var STEP_DWELL_MS=4000;
var STEP_WAIT_MS=8000;
var ERRORS_MAX=20;
var ERROR_LENGTH_MAX=300;
function canonicalFields(fields){
var pairs=[];
fields.forEach(function(field){
if(field[1]!=='')pairs.push(field[0]+'='+field[1]);
});
pairs.sort();
return pairs.join('&');
}
function exchangeKey(method,path,query,form,htmx){
var key=method.toUpperCase()+' '+path;
var fields=canonicalFields(query);
if(fields)key+='?'+fields;
fields=canonicalFields(form);
if(fields)key+=' form '+fields;
if(htmx)key+=' htmx';
return key;
}
function fieldsOf(collection){
var fields=[];
collection.forEach(function(value,name){
if(typeof value==='string')fields.push([name,value]);
});
return fields;
}
function newElement(name,className,text){
var node=document.createElement(name);
if(className)node.className=className;
if(text)node.textContent=text;
return node;
}
function chunkServer(mount,frameSource,report){
var files={};
var declared=mount.getAttribute('data-lg-replay-chunks');
if(declared){
try{
var parsed=JSON.parse(declared);
if(parsed&&typeof parsed==='object'&&!Array.isArray(parsed))files=parsed;
else report('data-lg-replay-chunks is not an object from chunk name to file');
}catch(error){
report('data-lg-replay-chunks cannot be read: '+error.message);
}
}
var loads={};
return function(target,name){
function answer(message){
message.from=FROM_HOST;
message.name=name;
target.postMessage(message,'*');
}
if(typeof name!=='string'||!Object.prototype.hasOwnProperty.call(files,name)){
answer({type:'chunk-failed',reason:'the page names no chunk '+String(name)});
return;
}
var file=files[name];
if(typeof file!=='string'||!CHUNK_FILE.test(file)){
answer({type:'chunk-failed',reason:'the page names no chunk file for '+name});
return;
}
if(!loads[name]){
var address=new URL(file,new URL(frameSource,document.baseURI));
loads[name]=fetch(address).then(function(response){
if(!response.ok)throw new Error(file+' answered '+response.status);
return response.json();
}).then(function(exchanges){
if(!Array.isArray(exchanges))throw new Error(file+' holds no list of exchanges');
return exchanges;
});
loads[name].catch(function(){delete loads[name];});
}
loads[name].then(function(exchanges){
answer({type:'chunk',exchanges:exchanges});
},function(error){
answer({type:'chunk-failed',reason:String(error&&error.message?error.message:error)});
});
};
}
function errorKeeper(mount){
var kept=0;
var dropped=0;
return function(text){
if(kept===ERRORS_MAX){
dropped+=1;
mount.setAttribute('data-lg-replay-errors-dropped',String(dropped));
return;
}
kept+=1;
var list=mount.getAttribute('data-lg-replay-errors');
var line=text.slice(0,ERROR_LENGTH_MAX);
mount.setAttribute('data-lg-replay-errors',list?list+' | '+line:line);
};
}
function mountFrame(mount){
var frameSource=mount.getAttribute('data-lg-replay-frame');
var fill=mount.hasAttribute('data-lg-replay-fill');
var keepError=errorKeeper(mount);
var serveChunk=chunkServer(mount,frameSource,keepError);
var frame=null;
var loads=0;
function loadFrame(key){
var fresh=document.createElement('iframe');
fresh.className='lg-replay__frame';
fresh.title=mount.getAttribute('data-lg-replay-title')||'Recorded replay';
fresh.setAttribute('sandbox','allow-scripts allow-forms');
fresh.src=frameSource+(key?'#'+encodeURIComponent(key):'');
if(fill){
fresh.style.display='block';
fresh.style.width='100%';
fresh.style.height='100%';
fresh.style.border='0';
}
mount.setAttribute('data-lg-replay-state','loading');
mount.removeAttribute('data-lg-replay-page');
if(frame)mount.replaceChild(fresh,frame);
else mount.appendChild(fresh);
frame=fresh;
}
while(mount.firstChild)mount.removeChild(mount.firstChild);
loadFrame('');
window.addEventListener('message',function(event){
if(event.source!==frame.contentWindow)return;
var message=event.data;
if(!message||message.from!==FROM_FRAME)return;
if(message.type==='ready'){
loads+=1;
mount.setAttribute('data-lg-replay-state','ready');
mount.setAttribute('data-lg-replay-page',message.key);
mount.setAttribute('data-lg-replay-loads',String(loads));
}else if(message.type==='open'){
loadFrame(message.key);
}else if(message.type==='chunk'){
serveChunk(frame.contentWindow,message.name);
}else if(message.type==='answered'){
mount.setAttribute('data-lg-replay-answered',message.key);
}else if(message.type==='missing'){
mount.setAttribute('data-lg-replay-missing',message.request);
}else if(message.type==='error'){
keepError(String(message.text));
}else if(message.type==='refused'){
mount.setAttribute('data-lg-replay-state','refused');
mount.appendChild(newElement('p','lg-replay__status',String(message.reason)));
}
});
}
function mountPlayer(mount){
var section=mount.closest('.lg-replay')||mount.parentNode;
var steps=Array.prototype.slice.call(section.querySelectorAll('[data-lg-replay-step]'));
var state={
step:-1,current:-1,target:-1,playing:false,busy:false,ready:false,opening:false,
page:'',progress:'',recoveredStep:-1,loads:0,
timer:0,watchdog:0
};
var keepError=errorKeeper(mount);
var controls=newElement('div','lg-replay__controls');
controls.setAttribute('role','group');
controls.setAttribute('aria-label','Tour of the recorded looking glass');
var play=newElement('button','lg-replay__button','Play tour');
var next=newElement('button','lg-replay__button','Next step');
var restart=newElement('button','lg-replay__button','Restart');
[play,next,restart].forEach(function(button){
button.type='button';
controls.appendChild(button);
});
play.setAttribute('aria-pressed','false');
var caption=newElement('p','lg-replay__caption',
'The first recorded page is open. Click through it, or play the tour.');
var status=newElement('p','lg-replay__status');
status.setAttribute('role','status');
status.setAttribute('aria-live','polite');
var answer=newElement('pre','lg-replay__answer');
answer.hidden=true;
answer.tabIndex=0;
answer.setAttribute('aria-label','Recorded text answer');
var frameSource=mount.getAttribute('data-lg-replay-frame');
var serveChunk=chunkServer(mount,frameSource,keepError);
var frame=null;
function loadFrame(key){
var fresh=document.createElement('iframe');
fresh.className='lg-replay__frame';
fresh.title='Recorded Ze looking glass';
fresh.setAttribute('sandbox','allow-scripts allow-forms');
fresh.src=frameSource+(key?'#'+encodeURIComponent(key):'');
state.ready=false;
state.page='';
mount.setAttribute('data-lg-replay-state','loading');
mount.removeAttribute('data-lg-replay-page');
if(frame)mount.replaceChild(fresh,frame);
else mount.appendChild(fresh);
frame=fresh;
}
while(mount.firstChild)mount.removeChild(mount.firstChild);
mount.appendChild(controls);
mount.appendChild(caption);
mount.appendChild(status);
loadFrame('');
mount.appendChild(answer);
function setPlaying(playing){
state.playing=playing;
play.textContent=playing?'Pause tour':'Play tour';
play.setAttribute('aria-pressed',playing?'true':'false');
if(!playing)window.clearTimeout(state.timer);
}
function markStep(index){
steps.forEach(function(item,position){
if(position===index)item.setAttribute('aria-current','step');
else item.removeAttribute('aria-current');
});
}
var lastStep=steps.length-1;
var tourOver='The tour is over. Press Restart to play it again.';
function showProgress(){
next.setAttribute('aria-busy',state.busy?'true':'false');
if(!state.busy){
if(status.textContent===state.progress)status.textContent='';
state.progress='';
return;
}
var queued=state.target-state.current;
state.progress='Step '+(state.current+1)+' is loading'+
(queued>0?', and '+queued+' more '+(queued===1?'press waits':'presses wait')+' behind it.':'.');
status.textContent=state.progress;
}
function playStep(index){
var text=steps[index].querySelector('[data-lg-replay-caption]');
caption.textContent='Step '+(index+1)+' of '+steps.length+': '+
(text?text.textContent:'');
answer.hidden=true;
markStep(index);
state.busy=true;
state.current=index;
mount.setAttribute('data-lg-replay-step-state','playing');
showProgress();
window.clearTimeout(state.watchdog);
state.watchdog=window.setTimeout(function(){
if(!state.busy)return;
state.busy=false;
state.target=state.step;
setPlaying(false);
showProgress();
status.textContent='Step '+(index+1)+' did not finish. Press Restart to play the tour from its first step.';
},STEP_WAIT_MS);
frame.contentWindow.postMessage({from:FROM_HOST,type:'step',index:index},'*');
}
function advance(){
if(state.busy){
showProgress();
return;
}
if(!state.ready)return;
if(state.step>=state.target)return;
playStep(state.step+1);
}
function stepDone(){
if(!state.busy)return;
state.busy=false;
window.clearTimeout(state.watchdog);
state.step=state.current;
if(state.recoveredStep===state.step)state.recoveredStep=-1;
showProgress();
mount.setAttribute('data-lg-replay-step-state','done');
mount.setAttribute('data-lg-replay-step',String(state.step));
if(state.step<state.target){
advance();
return;
}
if(!state.playing)return;
if(state.step===lastStep){
setPlaying(false);
status.textContent=tourOver;
return;
}
state.timer=window.setTimeout(function(){
state.target=state.step+1;
advance();
},STEP_DWELL_MS);
}
function restartTour(){
window.clearTimeout(state.timer);
window.clearTimeout(state.watchdog);
state.busy=false;
state.opening=false;
state.step=-1;
state.current=-1;
state.target=0;
state.recoveredStep=-1;
showProgress();
status.textContent='';
answer.hidden=true;
mount.removeAttribute('data-lg-replay-step');
loadFrame('');
}
play.addEventListener('click',function(){
setPlaying(!state.playing);
if(!state.playing)return;
if(state.step>=lastStep){
restartTour();
return;
}
if(state.target<=state.step)state.target=state.step+1;
advance();
});
next.addEventListener('click',function(){
setPlaying(false);
if(state.target>=lastStep){
if(state.busy)showProgress();
else if(state.step>=lastStep)status.textContent=tourOver;
return;
}
state.target+=1;
advance();
});
restart.addEventListener('click',function(){
setPlaying(false);
restartTour();
});
window.addEventListener('message',function(event){
if(event.source!==frame.contentWindow)return;
var message=event.data;
if(!message||message.from!==FROM_FRAME)return;
if(message.type==='ready'){
state.ready=true;
state.page=message.key;
state.loads+=1;
mount.setAttribute('data-lg-replay-state','ready');
mount.setAttribute('data-lg-replay-page',message.key);
mount.setAttribute('data-lg-replay-loads',String(state.loads));
if(state.opening){
state.opening=false;
stepDone();
}else{
advance();
}
}else if(message.type==='open'){
if(message.step&&state.page===message.key){
stepDone();
return;
}
state.opening=Boolean(message.step);
loadFrame(message.key);
}else if(message.type==='step-done'){
stepDone();
}else if(message.type==='chunk'){
serveChunk(frame.contentWindow,message.name);
}else if(message.type==='text'){
answer.textContent=message.body;
answer.hidden=false;
stepDone();
}else if(message.type==='answered'){
state.page='';
mount.setAttribute('data-lg-replay-answered',message.key);
}else if(message.type==='missing'){
state.page='';
mount.setAttribute('data-lg-replay-missing',message.request);
status.textContent='Not in this recording: '+message.request+
'. The replay holds the answers of the tour and of the rows its pages show.';
}else if(message.type==='unreachable'){
window.clearTimeout(state.watchdog);
state.busy=false;
if(state.recoveredStep!==state.current){
state.recoveredStep=state.current;
state.step=-1;
showProgress();
status.textContent='Returning to the page step '+(state.current+1)+' starts from.';
state.progress=status.textContent;
state.current=-1;
loadFrame('');
return;
}
state.recoveredStep=-1;
state.target=state.step;
setPlaying(false);
showProgress();
status.textContent='The control step '+(state.current+1)+' presses is not on the page the earlier steps leave. '+
'Press Restart to play the tour from its first step.';
}else if(message.type==='error'){
keepError(String(message.text));
}else if(message.type==='refused'){
mount.setAttribute('data-lg-replay-state','refused');
status.textContent=message.reason;
}
});
}
function hostMain(){
Array.prototype.forEach.call(document.querySelectorAll('[data-lg-replay]'),function(mount){
var section=mount.closest('.lg-replay')||mount.parentNode;
if(section.querySelector('[data-lg-replay-step]'))mountPlayer(mount);
else mountFrame(mount);
});
}
function frameMain(recordingElement){
function tellHost(message){
message.from=FROM_FRAME;
window.parent.postMessage(message,'*');
}
window.addEventListener('error',function(event){
tellHost({type:'error',text:'error: '+event.message+' (line '+event.lineno+')'});
});
window.addEventListener('unhandledrejection',function(event){
tellHost({type:'error',text:'unhandled rejection: '+String(event.reason)});
});
document.addEventListener('securitypolicyviolation',function(event){
tellHost({type:'error',text:'refused by the frame policy: '+event.violatedDirective+' '+event.blockedURI});
});
var recording;
try{
recording=JSON.parse(recordingElement.textContent);
}catch(error){
tellHost({type:'refused',reason:'The recording in this page cannot be read: '+error.message});
return;
}
if(recording.schema!==SCHEMA){
tellHost({type:'refused',reason:'The recording has schema '+recording.schema+
', and this player reads schema '+SCHEMA+'.'});
return;
}
var settings=recording.player||{};
var assetPrefixes=settings['asset-prefixes']||DEFAULT_ASSET_PREFIXES;
var noticeText=settings['notice-text']||DEFAULT_NOTICE;
var noticeClass=settings['notice-class']||DEFAULT_NOTICE_CLASS;
var exchanges={};
recording.exchanges.forEach(function(exchange){exchanges[exchange.key]=exchange;});
var chunkOf={};
var chunkConflict='';
(recording.chunks||[]).forEach(function(chunk){
(chunk.keys||[]).forEach(function(key){
if(exchanges[key]||chunkOf[key])chunkConflict=chunkConflict||key;
chunkOf[key]=chunk.name;
});
});
if(chunkConflict){
tellHost({type:'refused',reason:'The recording holds '+chunkConflict+' twice, so the replay cannot tell which answer is recorded.'});
return;
}
var fallbacks=recording.fallbacks||[];
var assets={};
recording.assets.forEach(function(asset){assets[asset.path]=asset;});
var stepExchangeKey='';
function isMarkup(answer){
return answer['content-type'].indexOf('text/html')===0;
}
function dataURI(asset){
return'data:'+asset['content-type'].split(';')[0]+';charset=utf-8,'+encodeURIComponent(asset.body);
}
function isAssetPath(path){
return assetPrefixes.some(function(prefix){
return path.length>prefix.length&&path.indexOf(prefix)===0;
});
}
function inlineImages(markup){
return markup.replace(/\bsrc="(\/[^"]+)"/g,function(whole,path){
if(!isAssetPath(path))return whole;
var asset=assets[path];
if(!asset||asset['content-type'].indexOf('image/')!==0)return whole;
return'src="'+dataURI(asset)+'"';
});
}
function requestLabel(key){
return key.replace(/ htmx$/,'');
}
function reportMissing(key){
tellHost({type:'missing',request:requestLabel(key)});
}
var chunkLoads={};
function loadChunk(name){
if(chunkLoads[name])return chunkLoads[name].promise;
var pending={settled:false};
pending.promise=new Promise(function(resolve,reject){
pending.resolve=resolve;
pending.reject=reject;
});
pending.timer=window.setTimeout(function(){
settleChunk(name,'the page did not hand it over within '+(CHUNK_WAIT_MS/1000)+' seconds');
},CHUNK_WAIT_MS);
chunkLoads[name]=pending;
tellHost({type:'chunk',name:name});
return pending.promise;
}
function settleChunk(name,failure){
var pending=chunkLoads[name];
if(!pending||pending.settled)return;
pending.settled=true;
window.clearTimeout(pending.timer);
if(failure){
delete chunkLoads[name];
pending.reject(failure);
return;
}
pending.resolve();
}
function chunkArrived(name,held){
var pending=chunkLoads[name];
if(!pending||pending.settled)return;
if(!Array.isArray(held)){
settleChunk(name,'the page handed over no list of answers');
return;
}
held.forEach(function(exchange){
var key=exchange&&exchange.key;
if(typeof key==='string'&&chunkOf[key]===name){
exchanges[key]=exchange;
return;
}
tellHost({type:'error',text:'chunk '+name+' holds '+String(key)+', which the recording does not list in it'});
});
settleChunk(name,'');
}
function chunkFailure(name,reason){
return'The part of the recording named '+name+' could not be loaded: '+reason+'.';
}
function withExchange(key,found,missing){
if(exchanges[key]){
found(exchanges[key]);
return;
}
var name=chunkOf[key];
if(!name){
missing('');
return;
}
loadChunk(name).then(function(){
if(exchanges[key])found(exchanges[key]);
else missing('');
},function(reason){
missing(chunkFailure(name,String(reason)));
});
}
function fallbackFor(request){
for(var index=0;index<fallbacks.length;index++){
var fallback=fallbacks[index];
if(fallback.method===request.method&&request.path.indexOf(fallback['path-prefix'])===0)return fallback;
}
return null;
}
function memoryStorage(){
var values={};
return{
getItem:function(name){
return Object.prototype.hasOwnProperty.call(values,name)?values[name]:null;
},
setItem:function(name,value){values[name]=String(value);},
removeItem:function(name){delete values[name];}
};
}
['localStorage','sessionStorage'].forEach(function(name){
Object.defineProperty(window,name,{configurable:true,value:memoryStorage()});
});
var pushedAddress='';
if(settings['keep-pushed-address']){
var keepAddress=function(state,title,url){
if(url===undefined||url===null)return;
pushedAddress=new URL(String(url),addressBase()).href;
};
window.history.pushState=keepAddress;
window.history.replaceState=keepAddress;
}else{
window.history.pushState=function(){};
window.history.replaceState=function(){};
}
function addressBase(){
return pushedAddress||window.location.href;
}
if(settings['silent-event-source']){
class SilentEventSource extends EventTarget{
constructor(url,options){
super();
var source=this;
this.url=new URL(String(url),addressBase()).href;
this.withCredentials=Boolean(options&&options.withCredentials);
this.readyState=SilentEventSource.CONNECTING;
this.onopen=null;
this.onmessage=null;
this.onerror=null;
['open','message','error'].forEach(function(type){
source.addEventListener(type,function(event){
var handler=source['on'+type];
if(typeof handler==='function')handler.call(source,event);
});
});
window.setTimeout(function(){
if(source.readyState!==SilentEventSource.CONNECTING)return;
source.readyState=SilentEventSource.OPEN;
source.dispatchEvent(new Event('open'));
},0);
}
close(){
this.readyState=SilentEventSource.CLOSED;
}
}
[['CONNECTING',0],['OPEN',1],['CLOSED',2]].forEach(function(state){
SilentEventSource[state[0]]=state[1];
SilentEventSource.prototype[state[0]]=state[1];
});
window.EventSource=SilentEventSource;
}
function requestOf(input,init){
var raw=typeof input==='string'?input:(input&&input.url)?input.url:String(input);
var target=new URL(raw,addressBase());
var method=(init.method||(input&&input.method)||'GET').toUpperCase();
var headers=new Headers(init.headers||(input&&input.headers)||{});
var form=[];
if(typeof init.body==='string')form=fieldsOf(new URLSearchParams(init.body));
else if(init.body&&typeof init.body.forEach==='function')form=fieldsOf(init.body);
return{
method:method,
path:target.pathname,
htmx:headers.get('HX-Request')==='true',
key:exchangeKey(method,target.pathname,fieldsOf(target.searchParams),form,
headers.get('HX-Request')==='true')
};
}
function streamResponse(signal){
var encoder=new TextEncoder();
var timers=[];
function stop(){timers.forEach(function(timer){window.clearTimeout(timer);});}
var body=new ReadableStream({
start:function(controller){
recording.stream.messages.forEach(function(message,index){
timers.push(window.setTimeout(function(){
try{
controller.enqueue(encoder.encode(inlineImages(message)+'\n\n'));
}catch(error){
}
},STREAM_INTERVAL_MS*(index+1)));
});
if(signal){
signal.addEventListener('abort',function(){
stop();
try{controller.close();}catch(error){}
});
}
},
cancel:stop
});
return new Response(body,{
status:200,
headers:{'Content-Type':recording.stream['content-type']||'text/event-stream'}
});
}
function noticeResponse(request,failure){
if(!request.htmx){
return new Response('Not in this recording: '+requestLabel(request.key)+'\n'+(failure?failure+'\n':''),
{status:404,headers:{'Content-Type':'text/plain; charset=utf-8'}});
}
var notice=document.createElement('div');
notice.className=noticeClass;
notice.setAttribute('role','status');
notice.textContent=noticeText.split(NOTICE_REQUEST).join(requestLabel(request.key))+(failure?' '+failure:'');
return new Response(notice.outerHTML,
{status:200,headers:{'Content-Type':'text/html; charset=utf-8'}});
}
function answerResponse(exchange){
tellHost({type:'answered',key:exchange.key});
if(stepExchangeKey===exchange.key){
stepExchangeKey='';
window.setTimeout(function(){tellHost({type:'step-done'});},150);
}
var body=isMarkup(exchange)?inlineImages(exchange.body):exchange.body;
return new Response(body,
{status:exchange.status,headers:{'Content-Type':exchange['content-type']}});
}
function unrecordedResponse(request,failure){
var fallback=failure?null:fallbackFor(request);
if(fallback)return answerResponse(fallback.exchange);
reportMissing(request.key);
return noticeResponse(request,failure);
}
window.fetch=function(input,init){
var request=requestOf(input,init||{});
if(recording.stream&&request.method==='GET'&&request.path===recording.stream.path){
return Promise.resolve(streamResponse(init&&init.signal));
}
var exchange=exchanges[request.key];
if(exchange)return Promise.resolve(answerResponse(exchange));
return new Promise(function(resolve){
withExchange(request.key,function(found){
resolve(answerResponse(found));
},function(failure){
resolve(unrecordedResponse(request,failure));
});
});
};
function openDocument(key,step){
tellHost({type:'open',key:key,step:step});
}
function openRecorded(key){
withExchange(key,function(exchange){
if(isMarkup(exchange))openDocument(key,false);
else reportMissing(key);
},function(){
reportMissing(key);
});
}
document.addEventListener('click',function(event){
var link=event.target&&event.target.closest?event.target.closest('a[href]'):null;
if(!link)return;
if(link.hasAttribute('hx-get'))return;
if(link.hasAttribute('hx-post'))return;
event.preventDefault();
var target=new URL(link.getAttribute('href'),addressBase());
openRecorded(exchangeKey('GET',target.pathname,fieldsOf(target.searchParams),[],false));
},true);
if(settings['plain-forms']){
window.addEventListener('submit',function(event){
var form=event.target;
if(event.defaultPrevented)return;
if(form.hasAttribute('hx-post'))return;
if(form.hasAttribute('hx-get'))return;
var method=(form.getAttribute('method')||'GET').toUpperCase();
if(method==='DIALOG')return;
event.preventDefault();
var target=new URL(form.getAttribute('action')||'',addressBase());
if(method!=='GET'){
reportMissing(method+' '+target.pathname);
return;
}
openRecorded(exchangeKey('GET',target.pathname,fieldsOf(new FormData(form,event.submitter)),[],false));
});
}else{
document.addEventListener('submit',function(event){
var form=event.target;
if(form.hasAttribute('hx-post'))return;
if(form.hasAttribute('hx-get'))return;
event.preventDefault();
reportMissing((form.getAttribute('method')||'GET').toUpperCase()+' '+(form.getAttribute('action')||''));
},true);
}
function showDocument(key){
withExchange(key,function(exchange){
putDocument(key,exchange);
},function(failure){
document.body.textContent='Not in this recording: '+requestLabel(key)+(failure?' '+failure:'');
reportMissing(key);
});
}
function putDocument(key,exchange){
var recordedDocument=new DOMParser().parseFromString(exchange.body,'text/html');
var scripts=[];
var deferred=[];
var lang=recordedDocument.documentElement.getAttribute('lang');
if(lang)document.documentElement.setAttribute('lang',lang);
document.title=recordedDocument.title;
Array.prototype.forEach.call(recordedDocument.head.children,function(node){
var name=node.nodeName.toLowerCase();
if(name==='link'&&node.getAttribute('rel')==='stylesheet'){
var sheet=assets[node.getAttribute('href')];
if(!sheet){
reportMissing('GET '+node.getAttribute('href'));
return;
}
document.head.appendChild(newElement('style','',sheet.body));
}else if(name==='script'&&node.hasAttribute('src')){
var script=assets[node.getAttribute('src')];
if(!script){
reportMissing('GET '+node.getAttribute('src'));
return;
}
if(node.hasAttribute('defer')||node.getAttribute('type')==='module')deferred.push(script.body);
else scripts.push(script.body);
}else if(name==='script'){
scripts.push(node.textContent);
}else if(name==='style'){
document.head.appendChild(document.importNode(node,true));
}
});
Array.prototype.forEach.call(recordedDocument.querySelectorAll('img[src]'),function(image){
var asset=assets[image.getAttribute('src')];
if(asset)image.setAttribute('src',dataURI(asset));
});
document.documentElement.replaceChild(document.importNode(recordedDocument.body,true),document.body);
scripts.concat(deferred).forEach(function(text){
document.head.appendChild(newElement('script','',text));
});
if(document.readyState!=='loading'){
document.dispatchEvent(new Event('DOMContentLoaded',{bubbles:true}));
}
if(document.readyState==='complete'){
window.dispatchEvent(new Event('load'));
}
tellHost({type:'ready',key:key});
}
function findControl(key){
var controls=document.querySelectorAll('[hx-get]');
for(var index=0;index<controls.length;index++){
var target=new URL(controls[index].getAttribute('hx-get'),window.location.href);
if(exchangeKey('GET',target.pathname,fieldsOf(target.searchParams),[],true)===key){
return controls[index];
}
}
return null;
}
function findForm(path){
var forms=document.querySelectorAll('form[hx-post]');
for(var index=0;index<forms.length;index++){
if(new URL(forms[index].getAttribute('hx-post'),window.location.href).pathname===path){
return forms[index];
}
}
return null;
}
function playStep(index){
var step=recording.tour[index];
var exchange=step?exchanges[step.exchange]:null;
if(!exchange){
tellHost({type:'unreachable'});
return;
}
if(!exchange.htmx){
if(isMarkup(exchange))openDocument(exchange.key,true);
else tellHost({type:'text',key:exchange.key,body:exchange.body});
return;
}
if(exchange.method==='GET'){
var control=findControl(exchange.key);
if(!control){
tellHost({type:'unreachable'});
return;
}
stepExchangeKey=exchange.key;
control.scrollIntoView({block:'nearest'});
control.click();
return;
}
var form=findForm(exchange.path);
if(!form){
tellHost({type:'unreachable'});
return;
}
var values={};
(exchange.form||[]).forEach(function(field){values[field.name]=field.value;});
Array.prototype.forEach.call(form.elements,function(field){
if(!field.name)return;
field.value=Object.prototype.hasOwnProperty.call(values,field.name)?values[field.name]:'';
});
stepExchangeKey=exchange.key;
form.requestSubmit();
}
window.addEventListener('message',function(event){
if(event.source!==window.parent)return;
var message=event.data;
if(!message||message.from!==FROM_HOST)return;
if(message.type==='step')playStep(message.index);
else if(message.type==='chunk')chunkArrived(message.name,message.exchanges);
else if(message.type==='chunk-failed')settleChunk(message.name,String(message.reason));
});
var openingKey=recording.entry;
if(window.location.hash.length>1)openingKey=decodeURIComponent(window.location.hash.slice(1));
showDocument(openingKey);
}
function frameRefusal(){
if(window.parent===window)return'opened outside its page';
if(window.origin!=='null')return'framed without a sandbox';
return'';
}
var recordingElement=document.getElementById('lg-replay-recording');
if(recordingElement){
var refusal=frameRefusal();
if(refusal){
document.documentElement.setAttribute('data-lg-replay-frame','refused: '+refusal);
}else{
frameMain(recordingElement);
}
}else if(document.readyState==='loading'){
document.addEventListener('DOMContentLoaded',hostMain);
}else{
hostMain();
}
})();
