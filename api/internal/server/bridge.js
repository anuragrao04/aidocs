function(){
const appOrigin=window.__AIDOCS_APP_ORIGIN__||'*';
const frame=document.getElementById('aidocs-doc');
function doc(){try{return frame.contentDocument||frame.contentWindow.document}catch(e){return null}}
function textNodes(root){const w=(doc()||document).createTreeWalker(root,NodeFilter.SHOW_TEXT,{acceptNode:n=>n.nodeValue.trim()?NodeFilter.FILTER_ACCEPT:NodeFilter.FILTER_REJECT});const a=[];let n;while(n=w.nextNode())a.push(n);return a}
function clear(){const d=doc();if(!d)return;d.querySelectorAll('mark.aidocs-mark').forEach(m=>m.replaceWith(...m.childNodes));d.body&&d.body.normalize()}
function norm(s){return (s||'').replace(/\s+/g,' ').trim()}
function markQuote(q,active){const d=doc();if(!d||!q)return 0;const nq=norm(q);if(!nq)return 0;for(const n of textNodes(d.body)){const raw=n.nodeValue;const nText=norm(raw);if(nText.indexOf(nq)<0)continue;const i=raw.indexOf(q);if(i>=0){const r=d.createRange();r.setStart(n,i);r.setEnd(n,Math.min(raw.length,i+q.length));const m=d.createElement('mark');m.className='aidocs-mark'+(active?' aidocs-mark-active':'');try{r.surroundContents(m);if(active)m.scrollIntoView({block:'center',behavior:'smooth'});return 1}catch(e){}}else{const m=d.createElement('mark');m.className='aidocs-mark'+(active?' aidocs-mark-active':'');try{const r=d.createRange();r.selectNode(n);r.surroundContents(m);if(active)m.scrollIntoView({block:'center',behavior:'smooth'});return 1}catch(e){}}}
return 0}
function paint(items,active){clear();(items||[]).forEach(x=>markQuote(x.quote||x.selected_text,x.id===active))}
function selection(){const d=doc();if(!d)return;const s=d.getSelection();if(!s||s.isCollapsed)return;const q=s.toString().trim();if(!q)return;let pre='',suf='',start=0,end=q.length,domPath='body';try{const body=d.body.innerText||d.body.textContent||'';start=body.indexOf(q);end=start+q.length;pre=body.slice(Math.max(0,start-64),start);suf=body.slice(end,end+64)}catch(e){}const targetOrigin=appOrigin==='self'?'*':appOrigin;parent.postMessage({type:'aidocs:selection',anchor:{quote:q,prefix:pre,suffix:suf,dom_path:domPath,start_offset:start,end_offset:end}},targetOrigin)}
frame.addEventListener('load',()=>{const d=doc();if(!d)return;d.addEventListener('mouseup',()=>setTimeout(selection,0));d.addEventListener('keyup',()=>setTimeout(selection,0));const targetOrigin=appOrigin==='self'?'*':appOrigin;parent.postMessage({type:'aidocs:ready'},targetOrigin)})
window.addEventListener('message',e=>{if(e.data&&e.data.type==='aidocs:paint')paint(e.data.comments,e.data.active)})
}
