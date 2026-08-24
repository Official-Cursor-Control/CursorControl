import { createClient } from "jsr:@supabase/supabase-js@2";

const SUPABASE_URL=Deno.env.get("SUPABASE_URL")!;
const SERVICE_ROLE=Deno.env.get("SUPABASE_SERVICE_ROLE_KEY")!;
const db=createClient(SUPABASE_URL,SERVICE_ROLE,{auth:{persistSession:false,autoRefreshToken:false}});
const H={"Content-Type":"application/json","Cache-Control":"no-store"};
function reply(x:unknown,status=200){return new Response(JSON.stringify(x),{status,headers:H});}
function cleanState(v:unknown):Record<string,unknown>{
  if(!v||typeof v!=="object"||Array.isArray(v)) return {};
  const out:Record<string,unknown>={};
  for(const [k,val] of Object.entries(v as Record<string,unknown>)){
    if(!k.startsWith("afk_")) continue;
    if(["afk_starbits","afk_lifetime_starbits","afk_offline_pending_starbits","afk_offline_pending_away_seconds","afk_offline_pending_paid_seconds"].includes(k)) continue;
    out[k]=val;
  }
  return out;
}
async function getUser(req:Request){
  const a=req.headers.get("Authorization")||""; const t=a.startsWith("Bearer ")?a.slice(7):"";
  if(!t) return null; const {data,error}=await db.auth.getUser(t); return error?null:data.user;
}

Deno.serve(async(req:Request)=>{
  if(req.method!=="POST") return reply({ok:false,message:"method not allowed"},405);
  const user=await getUser(req); if(!user) return reply({ok:false,message:"unauthorized"},401);
  let body:any={}; try{body=await req.json();}catch{return reply({ok:false,message:"invalid json"},400);}
  const action=String(body.action||"");
  const deviceId=String(body.device_id||"").trim().slice(0,160);
  if(!deviceId) return reply({ok:false,message:"missing device id"},400);
  const now=new Date().toISOString();

  if(action==="claim_device"){
    const sessionNonce=crypto.randomUUID();
    let sessionWrite=await db.from("active_game_sessions").upsert({
      user_id:user.id,device_id:deviceId,session_nonce:sessionNonce,device_name:"Cursor Control Windows",
      claimed_at:now,last_seen_at:now,updated_at:now
    },{onConflict:"user_id"});
    if(sessionWrite.error){
      await new Promise(r=>setTimeout(r,150));
      sessionWrite=await db.from("active_game_sessions").upsert({
        user_id:user.id,device_id:deviceId,session_nonce:sessionNonce,device_name:"Cursor Control Windows",
        claimed_at:now,last_seen_at:now,updated_at:now
      },{onConflict:"user_id"});
    }
    if(sessionWrite.error){ console.error("claim_device session write failed",sessionWrite.error); return reply({ok:false,retryable:true,message:"session claim temporarily unavailable"},503); }
    let {data:progress,error:pe}=await db.from("afk_starbit_progress").select("*").eq("user_id",user.id).maybeSingle();
    if(pe) return reply({ok:false,message:"progress load failed"},500);
    if(!progress){
      const state=cleanState(body.state);
      const ins=await db.from("afk_starbit_progress").insert({
        user_id:user.id,state,starbits:0,lifetime_starbits:0,starbits_per_second:0,production_per_sec_milli:0,
        afk_capacity_seconds:7200,pending_starbits:0,pending_away_seconds:0,pending_paid_seconds:0,
        last_server_checkpoint:now,revision:1,updated_at:now
      }).select("*").single();
      if(ins.error) return reply({ok:false,message:"progress bootstrap failed"},500);
      progress=ins.data;
    }
    return reply({ok:true,session_token:sessionNonce,server_time:now,protocol_version:6,cloud_authoritative:true,
      revision:Number(progress.revision||0),state:progress.state||{},starbits:String(progress.starbits??"0"),
      pending_starbits:"0",pending_away_seconds:0,pending_paid_seconds:0});
  }

  if(action!=="sync") return reply({ok:false,message:"invalid action"},400);
  const token=String(body.session_token||"").trim();
  const {data:session,error:se}=await db.from("active_game_sessions").select("device_id,session_nonce").eq("user_id",user.id).maybeSingle();
  if(se) return reply({ok:false,message:"session lookup failed"},500);
  if(!session||session.device_id!==deviceId||String(session.session_nonce)!==token)
    return reply({ok:false,session_lost:true,force_logout:true,message:"Account is active on another device"},409);
  const {data:progress,error:pe}=await db.from("afk_starbit_progress").select("state,revision,starbits").eq("user_id",user.id).maybeSingle();
  if(pe||!progress) return reply({ok:false,message:"progress unavailable"},500);
  const state=cleanState(body.state);
  const oldState=(progress.state&&typeof progress.state==="object")?progress.state:{};
  const hp=Math.max(0,Math.min(2,Number((oldState as any).afk_station_hp_bonus??(oldState as any).afk_survival_extra_lives??0)||0));
  state.afk_station_hp_bonus=hp; state.afk_survival_extra_lives=hp;
  const rev=Number(progress.revision||0)+1;
  let upd=await db.from("afk_starbit_progress").update({state,last_server_checkpoint:now,revision:rev,updated_at:now}).eq("user_id",user.id).select("state,revision,starbits").single();
  if(upd.error){
    await new Promise(r=>setTimeout(r,150));
    upd=await db.from("afk_starbit_progress").update({state,last_server_checkpoint:now,revision:rev,updated_at:now}).eq("user_id",user.id).select("state,revision,starbits").single();
  }
  if(upd.error){ console.error("progress save failed",upd.error); return reply({ok:false,retryable:true,message:"progress save temporarily unavailable"},503); }
  const seen=await db.from("active_game_sessions").update({last_seen_at:now,updated_at:now}).eq("user_id",user.id);
  if(seen.error) console.error("last_seen refresh failed",seen.error);
  return reply({ok:true,session_token:token,server_time:now,protocol_version:6,revision:rev,state:upd.data.state||{},
    starbits:String(upd.data.starbits??"0"),pending_starbits:"0",pending_away_seconds:0,pending_paid_seconds:0});
});
