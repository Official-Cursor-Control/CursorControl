import { createClient } from "npm:@supabase/supabase-js@2";

const H = {"content-type":"application/json; charset=utf-8","cache-control":"no-store","access-control-allow-origin":"*","access-control-allow-headers":"authorization, x-client-info, apikey, content-type","access-control-allow-methods":"POST, OPTIONS"};
const DIFFS = new Set(["EASY","NORMAL","HARD","INSANE","ENDURANCE"]);
const VN = 7*60*60*1000;
const EXP_RANKS = [
  ["BRONZE I",0],["BRONZE II",100],["BRONZE III",300],["SILVER I",750],["SILVER II",1500],["SILVER III",2500],
  ["GOLD I",4000],["GOLD II",6000],["GOLD III",8500],["PLATINUM I",12000],["PLATINUM II",16000],["PLATINUM III",22000],
  ["DIAMOND I",30000],["DIAMOND II",40000],["DIAMOND III",55000],["MASTER I",75000],["MASTER II",100000],["MASTER III",135000]
] as const;
function json(x:any,s=200){return new Response(JSON.stringify(x),{status:s,headers:H})}
function n(v:any,d=0){const x=Number(v);return Number.isFinite(x)?x:d}
function i(v:any,d=0){return Math.trunc(n(v,d))}
function clamp(v:number,a:number,b:number){return Math.max(a,Math.min(b,v))}
function rankForExp(exp:number){let rank="BRONZE I";for(const [name,threshold] of EXP_RANKS){if(exp>=threshold)rank=name;else break;}return rank;}
// v453: Monday 00:00 Asia/Ho_Chi_Minh -> next Monday 00:00. Sunday remains active.
function weekWindow(now=new Date()){
  const local=new Date(now.getTime()+VN);
  const dow=local.getUTCDay();
  const daysSinceMonday=(dow+6)%7;
  const startLocal=Date.UTC(local.getUTCFullYear(),local.getUTCMonth(),local.getUTCDate()-daysSinceMonday,0,0,0,0);
  const start=new Date(startLocal-VN);
  const end=new Date(start.getTime()+7*24*60*60*1000);
  const key=`VN-${new Date(startLocal).toISOString().slice(0,10)}`;
  return {key,start,end};
}
function displayName(user:any,fallback:any){const m=user?.user_metadata??{};return String(m.user_name??m.preferred_username??m.global_name??m.name??m.full_name??m.username??fallback??"PLAYER").trim().replace(/\s+/g," ").slice(0,32)||"PLAYER"}
function better(diff:string,a:any,b:any){if(!b)return true; if(a.score!==Number(b.score||0))return a.score>Number(b.score||0); if(diff==="ENDURANCE"){if(a.streak!==Number(b.streak||0))return a.streak>Number(b.streak||0);return a.accuracy>Number(b.accuracy||0)} if(a.accuracy!==Number(b.accuracy||0))return a.accuracy>Number(b.accuracy||0);return a.streak>Number(b.streak||0)}

Deno.serve(async(req:Request)=>{
  if(req.method==="OPTIONS")return json({ok:true});
  if(req.method!=="POST")return json({ok:false,error:"method not allowed"},405);
  const url=Deno.env.get("SUPABASE_URL")??"", service=Deno.env.get("SUPABASE_SERVICE_ROLE_KEY")??"";
  if(!url||!service)return json({ok:false,error:"server is not configured"},500);
  const admin=createClient(url,service,{auth:{persistSession:false,autoRefreshToken:false}});
  const token=(req.headers.get("authorization")??"").replace(/^Bearer\s+/i,"").trim();
  if(!token)return json({ok:false,error:"Discord login required"},401);
  const {data:au,error:ae}=await admin.auth.getUser(token); if(ae||!au.user)return json({ok:false,error:"invalid or expired session"},401);
  let body:any={}; try{body=await req.json()}catch{return json({ok:false,error:"invalid json"},400)}
  const difficulty=String(body.difficulty??"").trim().toUpperCase(); if(!DIFFS.has(difficulty))return json({ok:false,error:"invalid difficulty"},400);
  const score=clamp(i(body.score),0,2_000_000_000), streak=clamp(i(body.streak),0,10_000_000), accuracy=clamp(n(body.accuracy),0,100), targetCount=clamp(i(body.target_count),0,10_000_000), runTimeMs=clamp(i(body.run_time_ms),0,86400000), exp=clamp(i(body.exp),0,100_000_000);
  if(score<=0)return json({ok:false,error:"score must be greater than zero"},400);
  const name=displayName(au.user,body.display_name), isEndurance=difficulty==="ENDURANCE";
  const {data:oldPB,error:pbReadErr}=await admin.from("global_scores").select("*").eq("user_id",au.user.id).eq("difficulty",difficulty).maybeSingle();
  if(pbReadErr)return json({ok:false,error:"could not read existing personal best"},500);
  const {data:prog,error:progErr}=await admin.from("global_player_progress").select("*").eq("user_id",au.user.id).maybeSingle();
  if(progErr)return json({ok:false,error:"could not read player progression"},500);
  let easy=i(prog?.easy_clears),normal=i(prog?.normal_clears),hard=i(prog?.hard_clears),insane=i(prog?.insane_clears);
  if(!isEndurance){if(difficulty==="EASY")easy++; else if(difficulty==="NORMAL")normal++; else if(difficulty==="HARD")hard++; else if(difficulty==="INSANE")insane++;}

  const progressionExp=Math.max(i(prog?.exp),exp);
  const expRank=rankForExp(progressionExp);
  const {data:savedProgress,error:pw}=await admin.from("global_player_progress").upsert({user_id:au.user.id,display_name:name,exp:progressionExp,exp_rank:expRank,easy_clears:easy,normal_clears:normal,hard_clears:hard,insane_clears:insane,updated_at:new Date().toISOString()},{onConflict:"user_id"}).select("*").single();
  if(pw)return json({ok:false,error:"could not update player progression"},500);

  let savedPB=oldPB, pbSaved=false;
  const incoming={score,streak,accuracy};
  if(better(difficulty,incoming,oldPB)){
    const {data:r,error:e}=await admin.from("global_scores").upsert({user_id:au.user.id,display_name:name,difficulty,score,streak,accuracy,target_count:targetCount,run_time_ms:runTimeMs,exp:progressionExp,exp_rank:expRank,achieved_at:new Date().toISOString()},{onConflict:"user_id,difficulty"}).select("*").single();
    if(e)return json({ok:false,error:"could not save personal best"},500); savedPB=r;pbSaved=true;
  }else if(oldPB){const {data:r}=await admin.from("global_scores").update({display_name:name,exp:progressionExp,exp_rank:expRank}).eq("user_id",au.user.id).eq("difficulty",difficulty).select("*").single(); if(r)savedPB=r;}

  const wk=weekWindow();
  const {data:weeklyPB}=await admin.from("weekly_scores").select("*").eq("week_key",wk.key).eq("user_id",au.user.id).eq("difficulty",difficulty).maybeSingle();
  if(better(difficulty,incoming,weeklyPB)){
    await admin.from("weekly_scores").upsert({week_key:wk.key,user_id:au.user.id,difficulty,score,streak,accuracy,distance:isEndurance?score/10:0,targets_hit:isEndurance?streak:targetCount,achieved_at:new Date().toISOString()},{onConflict:"week_key,user_id,difficulty"});
  }
  let competition:any=null;
  if(!isEndurance){
    const {data:o}=await admin.from("weekly_scores").select("easy_clears,normal_clears,hard_clears,insane_clears").eq("week_key",wk.key).eq("user_id",au.user.id).eq("difficulty","OVERALL").maybeSingle();
    let we=i(o?.easy_clears),wn=i(o?.normal_clears),wh=i(o?.hard_clears),wi=i(o?.insane_clears);
    if(difficulty==="EASY")we++; else if(difficulty==="NORMAL")wn++; else if(difficulty==="HARD")wh++; else if(difficulty==="INSANE")wi++;
    const total=we+wn+wh+wi;
    await admin.from("weekly_scores").upsert({week_key:wk.key,user_id:au.user.id,difficulty:"OVERALL",score:total,easy_clears:we,normal_clears:wn,hard_clears:wh,insane_clears:wi,achieved_at:new Date().toISOString()},{onConflict:"week_key,user_id,difficulty"});

    // Competition scoring is entirely server-authoritative. The client never
    // submits points or a multiplier; the RPC owns per-difficulty streaks.
    const {data:comp,error:ce}=await admin.rpc("record_precision_competition_success",{p_user_id:au.user.id,p_difficulty:difficulty});
    if(!ce)competition=Array.isArray(comp)?comp[0]??null:comp;
  }
  return json({ok:true,difficulty,endurance:isEndurance,new_personal_best:pbSaved,personal_best:savedPB,progression:savedProgress,weekly:{week_key:wk.key,resets_at:wk.end.toISOString()},competition,submitted:{score,streak,accuracy,target_count:targetCount,run_time_ms:runTimeMs,exp:progressionExp,exp_rank:expRank}});
});
