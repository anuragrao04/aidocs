function(){
const appOrigin=window.__AIDOCS_APP_ORIGIN__||'*';
const frame=document.getElementById('aidocs-doc');
function doc(){try{return frame.contentDocument||frame.contentWindow.document}catch(e){return null}}
function textNodes(root){const w=(doc()||document).createTreeWalker(root,NodeFilter.SHOW_TEXT,{acceptNode:n=>n.nodeValue.trim()?NodeFilter.FILTER_ACCEPT:NodeFilter.FILTER_REJECT});const a=[];let n;while(n=w.nextNode())a.push(n);return a}
function clear(){const d=doc();if(!d)return;d.querySelectorAll('mark.aidocs-mark').forEach(m=>m.replaceWith(...m.childNodes));d.body&&d.body.normalize()}
function norm(s){return (s||'').replace(/\s+/g,' ').trim()}
function markQuote(q,active,id){const d=doc();if(!d||!q)return 0;const nq=norm(q);if(!nq)return 0;for(const n of textNodes(d.body)){const raw=n.nodeValue;const nText=norm(raw);if(nText.indexOf(nq)<0)continue;const i=raw.indexOf(q);if(i>=0){const r=d.createRange();r.setStart(n,i);r.setEnd(n,Math.min(raw.length,i+q.length));const m=d.createElement('mark');m.className='aidocs-mark'+(active?' aidocs-mark-active':'');if(id)m.dataset.cid=id;try{r.surroundContents(m);if(active)m.scrollIntoView({block:'center',behavior:'smooth'});return 1}catch(e){}}else{const m=d.createElement('mark');m.className='aidocs-mark'+(active?' aidocs-mark-active':'');if(id)m.dataset.cid=id;try{const r=d.createRange();r.selectNode(n);r.surroundContents(m);if(active)m.scrollIntoView({block:'center',behavior:'smooth'});return 1}catch(e){}}}
return 0}
function paint(items,active){clear();(items||[]).forEach(x=>markQuote(x.quote||x.selected_text,x.id===active,x.id))}
function activate(e){const m=e.target&&e.target.closest&&e.target.closest('mark.aidocs-mark');if(!m||!m.dataset.cid)return;const targetOrigin=appOrigin==='self'?'*':appOrigin;parent.postMessage({type:'aidocs:activate',id:m.dataset.cid},targetOrigin)}
var curTheme=null;
function initialTheme(){try{var t=new URLSearchParams(location.search).get('aidocs_theme');return t==='dark'||t==='light'?t:null}catch(e){return null}}
function applyTheme(t){if(t!=='dark'&&t!=='light')return;curTheme=t;const d=doc();if(!d||!d.documentElement)return;d.documentElement.setAttribute('data-aidocs-theme',t);try{d.defaultView.dispatchEvent(new CustomEvent('aidocs:theme',{detail:{theme:t}}))}catch(e){}}
function selection(){const d=doc();if(!d)return;const s=d.getSelection();if(!s||s.isCollapsed)return;const q=s.toString().trim();if(!q)return;let pre='',suf='',start=0,end=q.length,domPath='body';try{const body=d.body.innerText||d.body.textContent||'';start=body.indexOf(q);end=start+q.length;pre=body.slice(Math.max(0,start-64),start);suf=body.slice(end,end+64)}catch(e){}const targetOrigin=appOrigin==='self'?'*':appOrigin;parent.postMessage({type:'aidocs:selection',anchor:{quote:q,prefix:pre,suffix:suf,dom_path:domPath,start_offset:start,end_offset:end}},targetOrigin)}
curTheme=initialTheme();
frame.addEventListener('load',()=>{const d=doc();if(!d)return;applyTheme(curTheme);d.addEventListener('mouseup',()=>setTimeout(selection,0));d.addEventListener('keyup',()=>setTimeout(selection,0));d.addEventListener('click',activate);const targetOrigin=appOrigin==='self'?'*':appOrigin;parent.postMessage({type:'aidocs:ready'},targetOrigin)})
window.addEventListener('message',e=>{if(!e.data)return;if(e.data.type==='aidocs:paint')paint(e.data.comments,e.data.active);else if(e.data.type==='aidocs:theme')applyTheme(e.data.theme)})
}
