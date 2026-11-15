const $=s=>document.querySelector(s);let services=[];
async function load(){try{services=await (await fetch('/_portal/services.json',{cache:'no-store'})).json()}catch(e){return}render()}
function render(){
  const q=$('#q').value.toLowerCase(),g={};
  services.filter(s=>(s.title+s.group+s.port+s.name).toLowerCase().includes(q)).forEach(s=>(g[s.group]=g[s.group]||[]).push(s));
  $('#list').innerHTML=Object.entries(g).map(([n,a])=>`<div class="grp">${n}</div>`+a.map(s=>
   `<div class="item" data-p="${s.port}" data-path="${s.path||'/'}" title="${s.desc||''}"><span class="dot ${s.up===true?'up':s.up===false?'down':''}"></span><span class="t">${s.title}</span><span class="p">:${s.port}</span></div>`).join('')).join('');
}
async function open(port,path){
  $('#msg').textContent='opening :'+port+'…';
  try{const r=await fetch(`/_portal/api/open?port=${port}&path=${encodeURIComponent(path)}`,{headers:{'X-Portal':'1'}});
    const j=await r.json();$('#msg').textContent=r.ok?'':('error: '+(j.error||r.status))}catch(e){$('#msg').textContent='error: '+e}
  setTimeout(()=>{if($('#msg').textContent.startsWith('opening'))$('#msg').textContent=''},3000);
}
$('#list').onclick=e=>{const i=e.target.closest('.item');if(i)open(i.dataset.p,i.dataset.path)};
$('#q').oninput=render;
$('#reload').onclick=()=>{$('#vb').src='/browser/?t='+Date.now()};
$('#full').onclick=()=>window.open('/browser/','_blank');
load();setInterval(load,10000);
