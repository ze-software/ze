(function(){
"use strict";
var scriptMarker="assets/rfc-figures.js";
var root=null;
var indexPromise=null;
var filePromises={};
function siteRoot(){
if(root!==null)return root;
var script=document.querySelector('script[src*="'+scriptMarker+'"]');
var src=script?(script.getAttribute("src")||""):"";
var clean=src.split("#")[0].split("?")[0];
var at=clean.indexOf(scriptMarker);
root=at===-1?"":clean.slice(0,at);
return root;
}
var publish="Run ./le site build to publish data/rfc/.";
function unreadable(paths,consequence){
return new Error(paths.join(", ")+" could not be read, so "+consequence+". "+publish);
}
function loadFile(path){
if(!filePromises[path]){
filePromises[path]=fetch(siteRoot()+path).then(function(response){
if(!response.ok)throw new Error(path);
return response.json();
}).catch(function(){
throw new Error(path);
});
}
return filePromises[path];
}
function loadIndex(){
if(!indexPromise){
indexPromise=loadFile("data/rfc/index.json").catch(function(error){
throw unreadable([error.message],"no RFC figure on this page can be shown");
});
}
return indexPromise;
}
function splitMarker(value){
var at=String(value||"").lastIndexOf(":");
if(at<=0)return null;
return[value.slice(0,at),value.slice(at+1)];
}
function membersOf(index,subject,node){
if(subject==="enrolled"){
return Promise.resolve((index.rfcs||[]).filter(function(entry){
return entry.gated===true;
}).map(function(entry){return entry.stem;}));
}
if(subject==="all"){
var scope=node&&node.closest?node.closest("[data-ze-rfc-stems]"):null;
if(!scope){
return Promise.resolve((index.rfcs||[]).map(function(entry){return entry.stem;}));
}
return Promise.resolve(scope.getAttribute("data-ze-rfc-stems").split(",").filter(Boolean));
}
var parts=subject.split("/");
if(parts[0]!=="profile"){
return Promise.resolve([subject]);
}
if(parts.length!==3||(parts[2]!=="required"&&parts[2]!=="optional")){
return Promise.reject(new Error(subject+" is not a subject the RFC data answers, so nothing can be "+
"shown here. Fix the marker: write profile/<name>/required or profile/<name>/optional."));
}
var entry=(index.profiles||[]).filter(function(profile){
return profile.name===parts[1];
})[0];
if(!entry){
return Promise.reject(new Error("the index lists no profile named "+parts[1]+", so it has no "+
"figure. Fix the marker, or add rfc/profiles/"+parts[1]+".md. "+publish));
}
return loadFile(entry.file).then(function(profile){
return profile[parts[2]]||[];
},function(error){
throw unreadable([error.message],"the members of profile "+parts[1]+" are unknown");
});
}
function isCounts(value){
return value!==null&&typeof value==="object"&&!Array.isArray(value);
}
function addCounts(target,source){
Object.keys(source).forEach(function(key){
var value=source[key];
if(typeof value==="number"){
target[key]=(target[key]||0)+value;
return;
}
if(value&&typeof value==="object"&&!Array.isArray(value)){
target[key]=target[key]||{};
addCounts(target[key],value);
}
});
}
function sumStems(stems){
return loadIndex().then(function(index){
var files={};
(index.rfcs||[]).forEach(function(entry){
files[entry.stem]=entry.file;
});
var unknown=stems.filter(function(stem){return!files[stem];});
if(unknown.length){
throw new Error("the index lists no RFC named "+unknown.join(", ")+", so it has no figure. "+
"Fix the marker, or add its rfc/short/ summary. "+publish);
}
return Promise.all(stems.map(function(stem){
return loadFile(files[stem]).then(function(data){
return{data:data};
},function(){
return{missing:files[stem]};
});
}));
}).then(function(loaded){
var missing=loaded.filter(function(item){return item.missing;})
.map(function(item){return item.missing;});
if(missing.length){
throw unreadable(missing,"this figure would leave "+(missing.length===1?"that RFC":"those RFCs")+" out");
}
var sum={rfcs:loaded.length,walked:0,"not-walked":0,requirements:0,implemented:0,must:null,should:null};
var uncounted=loaded.filter(function(item){
return typeof item.data.requirements!=="number"||typeof item.data.implemented!=="boolean";
})
.map(function(item){return item.data.stem;});
if(uncounted.length){
throw new Error("the files of "+uncounted.join(", ")+" hold no requirements count or implemented flag, so this "+
"figure would leave their rows out. "+publish);
}
var unsplit=loaded.filter(function(item){
return item.data.walked&&!(isCounts(item.data.must)&&isCounts(item.data.should));
})
.map(function(item){return item.data.stem;});
if(unsplit.length){
throw new Error("the files of "+unsplit.join(", ")+" are walked but hold no must or should counts, so this "+
"figure would leave their rows out. "+publish);
}
loaded.forEach(function(item){
sum.requirements+=item.data.requirements;
if(item.data.implemented){
sum.implemented+=1;
}
if(!item.data.walked){
sum["not-walked"]+=1;
return;
}
sum.walked+=1;
sum.must=sum.must||{};
sum.should=sum.should||{};
addCounts(sum.must,item.data.must);
addCounts(sum.should,item.data.should);
});
sum.register=registerOf(sum);
return sum;
});
}
var registerOpenSections=["proof-pending","os-gap","exclusion-unchecked","not-assessed","not-built"];
function registerOf(sum){
if(sum.rfcs===0||sum["not-walked"]!==0||!sum.must)return"not complete";
for(var i=0;i<registerOpenSections.length;i++){
if(sum.must[registerOpenSections[i]]!==0)return"not complete";
}
return"complete";
}
function sumSubject(subject,node){
return loadIndex().then(function(index){
return membersOf(index,subject,node);
}).then(sumStems);
}
function lookup(sum,field){
var value=sum;
var parts=field.split(".");
for(var i=0;i<parts.length;i++){
if(value===null||typeof value!=="object")return undefined;
value=value[parts[i]];
}
return value;
}
var byKind=/^(must|should)\.not-owed-by-kind\.[^.]+$/;
function fieldsSum(sum,fields){
var total=0;
for(var i=0;i<fields.length;i++){
var value=lookup(sum,fields[i]);
if(typeof value!=="number")return NaN;
total+=value;
}
return total;
}
function badMarker(node,attribute,shape){
return attribute+"=\""+node.getAttribute(attribute)+"\" is not a marker the RFC data answers, so nothing can be shown here. "+
"Write it as "+shape+".";
}
function showError(node,message){
node.textContent=message;
node.classList.add("rfc-figure-error");
}
function fillValue(node){
var marker=splitMarker(node.getAttribute("data-ze-rfc"));
if(!marker)return showError(node,badMarker(node,"data-ze-rfc","<subject>:<field>"));
sumSubject(marker[0],node).then(function(sum){
var value=lookup(sum,marker[1]);
if(typeof value==="number"){
node.textContent=value.toLocaleString("en-GB");
return;
}
if(typeof value==="string"){
node.textContent=value;
return;
}
if(sum.rfcs===0){
node.textContent="no rows";
return;
}
if(sum.walked===0){
node.textContent="requirements not reviewed";
return;
}
if(value===undefined&&byKind.test(marker[1])){
node.textContent="0";
return;
}
showError(node,marker[1]+" is not a field of the RFC data, so nothing can be shown here. "+
"Fix the data-ze-rfc marker: docs/architecture/site-facts.md lists the fields.");
},function(error){
showError(node,error.message);
});
}
function drawBar(node,layout,sum,bar){
var segments=layout[bar];
var totalFields=layout[bar+"-total"];
if(!segments||!totalFields){
return showError(node,bar+" is not a bar the RFC data declares, so nothing can be drawn here. "+
"Write the marker as <subject>:must or <subject>:overview.");
}
if(sum.rfcs===0){
node.textContent="no rows";
return;
}
if(sum.walked===0){
node.textContent="requirements not reviewed";
return;
}
var total=fieldsSum(sum,totalFields);
var counts=segments.map(function(segment){return fieldsSum(sum,segment.fields);});
var drawn=counts.reduce(function(a,b){return a+b;},0);
if(!isFinite(total)||drawn!==total){
return showError(node,"the "+bar+" segments sum to "+drawn+", not the "+total+
" rows the files state, so the bar would misdraw them. "+publish);
}
while(node.firstChild)node.removeChild(node.firstChild);
node.classList.add("rfc-seg-figure");
if(total===0){
node.textContent="no rows";
return;
}
var track=document.createElement("div");
track.className="rfc-seg-bar";
var legend=document.createElement("ul");
legend.className="rfc-seg-legend";
segments.forEach(function(segment,index){
if(!counts[index])return;
var words=segment.label+" "+counts[index].toLocaleString("en-GB");
var span=document.createElement("span");
span.className="rfc-seg rfc-seg-"+segment.segment;
span.setAttribute("data-segment",segment.segment);
span.style.width=(100*counts[index]/total)+"%";
span.textContent=words;
span.title=words;
track.appendChild(span);
var item=document.createElement("li");
item.setAttribute("data-segment",segment.segment);
var swatch=document.createElement("span");
swatch.className="rfc-seg-swatch rfc-seg-"+segment.segment;
swatch.setAttribute("aria-hidden","true");
item.appendChild(swatch);
item.appendChild(document.createTextNode(words));
legend.appendChild(item);
});
node.appendChild(track);
node.appendChild(legend);
}
function fillBar(node){
var marker=splitMarker(node.getAttribute("data-ze-rfc-bar"));
if(!marker)return showError(node,badMarker(node,"data-ze-rfc-bar","<subject>:must or <subject>:overview"));
Promise.all([loadIndex(),sumSubject(marker[0],node)]).then(function(loaded){
drawBar(node,loaded[0].bars||{},loaded[1],marker[1]);
},function(error){
showError(node,error.message);
});
}
function fill(scope){
if(!scope||!window.fetch)return;
Array.prototype.forEach.call(scope.querySelectorAll("[data-ze-rfc]"),fillValue);
Array.prototype.forEach.call(scope.querySelectorAll("[data-ze-rfc-bar]"),fillBar);
}
window.zeRFCFigures={fill:fill,sumStems:sumStems};
if(document.readyState==="loading"){
document.addEventListener("DOMContentLoaded",function(){fill(document);});
}else{
fill(document);
}
})();
