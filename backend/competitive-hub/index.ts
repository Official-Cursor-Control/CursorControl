import { createClient } from "npm:@supabase/supabase-js@2";

const H={"content-type":"application/json; charset=utf-8","cache-control":"no-store","access-control-allow-origin":"*","access-control-allow-headers":"authorization, apikey, content-type","access-control-allow-methods":"GET, POST, OPTIONS"};
const DIFFS=new Set(["OVERALL","EASY","NORMAL","HARD","INSANE","ENDURANCE","SURVIVAL"]);
const VN=7*3600000;
const REWARDS=[{exp:5000,coins:1500},{exp:3000,coins:750},{exp:1500,coins:400}];
function json(x:any,s=200){return new Response(JSON.stringify(x),{status:s,headers:H})}
// Global competition/weekly cycle: Monday 00:00 Asia/Ho_Chi_Minh.
// Sunday is the final day of the active competition.
function weekWindow(now=new Date()){
  const l=new Date(now.getTime()+VN),dow=l.getUTCDay();
  const daysSinceMonday=(dow+6)%7;
  const sl=Date.UTC(l.getUTCFullYear(),l.getUTCMonth(),l.getUTCDate()-daysSinceMonday,0,0,0,0);
  const start=new Date(sl-VN),end=new Date(start.getTime()+7*86400000);
  return{key:`VN-${new Date(sl).toISOString().slice(0,10)}`,start,end};
}
async function enrich(admin:any,rows:any[]){
  const ids=[...new Set((rows??[]).map((r:any)=>r.user_id).filter(Boolean))];
  const [{data:progress},{data:profiles}]=ids.length?await Promise.all([
    admin.from("global_player_progress").select("user_id,display_name,exp_rank").in("user_id",ids),
    admin.from("global_player_profiles").select("user_id,selected_name_colour").in("user_id",ids)
  ]):[{data:[]},{data:[]}];
  return{pm:new Map((progress??[]).map((p:any)=>[p.user_id,p])),cm:new Map((profiles??[]).map((p:any)=>[p.user_id,p]))};
}
async function finalizePreviousPrecision(admin:any,wk:any){
  const prev=weekWindow(new Date(wk.start.getTime()-1000));
  const {error}=await admin.rpc("finalize_precision_competition_week",{p_week_key:prev.key});
  return{prev,error};
}
function competitionEntry(r:any,idx:number,p:any,c:any){return{
  position:idx+1,user_id:r.user_id,display_name:p.display_name||"PLAYER",exp_rank:p.exp_rank||"",selected_name_colour:c.selected_name_colour||0,
  total_points:Number(r.total_points||0),easy_points:Number(r.easy_points||0),normal_points:Number(r.normal_points||0),hard_points:Number(r.hard_points||0),insane_points:Number(r.insane_points||0),
  easy_streak:Number(r.easy_streak||0),normal_streak:Number(r.normal_streak||0),hard_streak:Number(r.hard_streak||0),insane_streak:Number(r.insane_streak||0),
  easy_completions:Number(r.easy_completions||0),normal_completions:Number(r.normal_completions||0),hard_completions:Number(r.hard_completions||0),insane_completions:Number(r.insane_completions||0),updated_at:r.updated_at
}}

Deno.serve(async(req:Request)=>{
  if(req.method==="OPTIONS")return json({ok:true});
  const url=Deno.env.get("SUPABASE_URL")??"",service=Deno.env.get("SUPABASE_SERVICE_ROLE_KEY")??"";
  if(!url||!service)return json({error:"server_config"},500);
  const admin=createClient(url,service,{auth:{persistSession:false,autoRefreshToken:false}});
  const token=(req.headers.get("authorization")??"").replace(/^Bearer\s+/i,"").trim();
  if(!token)return json({error:"unauthorized"},401);
  const {data:ud,error:ue}=await admin.auth.getUser(token);
  if(ue||!ud.user)return json({error:"unauthorized"},401);
  const uid=ud.user.id,wk=weekWindow();
  const {prev}=await finalizePreviousPrecision(admin,wk);

  if(req.method==="POST"){
    let body:any={};try{body=await req.json()}catch{return json({error:"invalid_json"},400)}
    const action=String(body.action??"").toLowerCase();
    if(action==="precision_failure"){
      const difficulty=String(body.difficulty??"").toUpperCase();
      if(!["EASY","NORMAL","HARD","INSANE"].includes(difficulty))return json({error:"invalid_difficulty"},400);
      const {error}=await admin.rpc("reset_precision_competition_streak",{p_user_id:uid,p_difficulty:difficulty});
      if(error)return json({error:"reset_failed",detail:error.message},500);
      return json({ok:true,week_key:wk.key,resets_at:wk.end.toISOString(),difficulty});
    }
    // Automatic delivery. Kept compatible with the old action name so an older
    // v446 pre-release cannot strand a pending award during rollout.
    if(action==="sync_precision_rewards"||action==="claim_precision_reward"){
      const {data,error}=await admin.rpc("collect_precision_competition_rewards",{p_user_id:uid});
      if(error)return json({error:"reward_sync_failed",detail:error.message},500);
      const r=Array.isArray(data)?data[0]??{}:data??{};
      const count=Number(r.award_count||0),coins=Number(r.spacecoins_reward||0);
      return json({ok:true,awarded:count>0,award_count:count,spacecoins_reward:coins,week_key:String(r.latest_week_key||""),placement:Number(r.latest_placement||0),exp_reward:Number(r.latest_exp_reward||0),global_exp:Number(r.global_exp||0),global_rank:String(r.global_rank||"BRONZE I"),resets_at:wk.end.toISOString()});
    }
    return json({week_key:wk.key,resets_at:wk.end.toISOString(),awarded:false});
  }
  if(req.method!=="GET")return json({error:"method_not_allowed"},405);

  const u=new URL(req.url),scope=(u.searchParams.get("scope")??"weekly").toLowerCase(),diff=(u.searchParams.get("difficulty")??"OVERALL").toUpperCase();
  if(scope==="competition"){
    const view=(u.searchParams.get("view")??"current").toLowerCase();
    if(view==="previous"){
      const {data:rows,error}=await admin.from("precision_competition_weekly")
        .select("user_id,total_points,easy_points,normal_points,hard_points,insane_points,easy_streak,normal_streak,hard_streak,insane_streak,easy_completions,normal_completions,hard_completions,insane_completions,updated_at")
        .eq("week_key",prev.key)
        .order("total_points",{ascending:false}).order("insane_points",{ascending:false}).order("hard_points",{ascending:false}).order("normal_points",{ascending:false}).order("easy_points",{ascending:false}).limit(3);
      if(error)return json({error:"previous_ranking_failed",detail:error.message},500);
      const {pm,cm}=await enrich(admin,rows??[]);
      const entries=(rows??[]).map((r:any,idx:number)=>{const p:any=pm.get(r.user_id)||{},c:any=cm.get(r.user_id)||{},base=competitionEntry(r,idx,p,c),reward=REWARDS[idx]??{exp:0,coins:0};return{...base,exp_reward:reward.exp,spacecoins_reward:reward.coins}});
      return json({week_key:prev.key,starts_at:prev.start.toISOString(),resets_at:wk.end.toISOString(),timezone:"Asia/Ho_Chi_Minh",scope:"competition",view:"previous",entries});
    }
    if(view==="alltime"){
      const {data:awards,error}=await admin.from("precision_competition_awards").select("user_id,placement,week_key,created_at").like("week_key","VN-%").order("created_at",{ascending:false}).limit(10000);
      if(error)return json({error:"winner_history_failed",detail:error.message},500);
      const by=new Map<string,any>();
      for(const a of awards??[]){
        const id=String(a.user_id||"");if(!id)continue;
        let x=by.get(id);if(!x){x={user_id:id,wins:0,podiums:0,firsts:0,seconds:0,thirds:0,last_win_week:""};by.set(id,x)}
        const place=Number(a.placement||0);x.podiums++;
        if(place===1){x.wins++;x.firsts++;if(!x.last_win_week)x.last_win_week=String(a.week_key||"")}
        else if(place===2)x.seconds++;else if(place===3)x.thirds++;
      }
      const rows=[...by.values()].filter((x:any)=>x.wins>0).sort((a:any,b:any)=>b.wins-a.wins||b.podiums-a.podiums||b.seconds-a.seconds||b.thirds-a.thirds||String(a.user_id).localeCompare(String(b.user_id))).slice(0,20);
      const {pm,cm}=await enrich(admin,rows);
      const entries=rows.map((r:any,idx:number)=>{const p:any=pm.get(r.user_id)||{},c:any=cm.get(r.user_id)||{};return{position:idx+1,user_id:r.user_id,display_name:p.display_name||"PLAYER",exp_rank:p.exp_rank||"",selected_name_colour:c.selected_name_colour||0,wins:r.wins,podiums:r.podiums,firsts:r.firsts,seconds:r.seconds,thirds:r.thirds,last_win_week:r.last_win_week}});
      return json({week_key:wk.key,resets_at:wk.end.toISOString(),timezone:"Asia/Ho_Chi_Minh",scope:"competition",view:"alltime",entries});
    }
    if(view!=="current")return json({error:"invalid_competition_view"},400);
    const {data:rows,error}=await admin.from("precision_competition_weekly")
      .select("user_id,total_points,easy_points,normal_points,hard_points,insane_points,easy_streak,normal_streak,hard_streak,insane_streak,easy_completions,normal_completions,hard_completions,insane_completions,updated_at")
      .eq("week_key",wk.key)
      .order("total_points",{ascending:false}).order("insane_points",{ascending:false}).order("hard_points",{ascending:false}).order("normal_points",{ascending:false}).order("easy_points",{ascending:false}).limit(20);
    if(error)return json({error:"competition_ranking_failed",detail:error.message},500);
    const {pm,cm}=await enrich(admin,rows??[]);
    const entries=(rows??[]).map((r:any,idx:number)=>{const p:any=pm.get(r.user_id)||{},c:any=cm.get(r.user_id)||{};return competitionEntry(r,idx,p,c)});
    return json({week_key:wk.key,starts_at:wk.start.toISOString(),resets_at:wk.end.toISOString(),timezone:"Asia/Ho_Chi_Minh",scope:"competition",view:"current",entries});
  }

  if(scope!=="weekly"||!DIFFS.has(diff))return json({error:"invalid_scope_or_difficulty"},400);
  let q:any=admin.from("weekly_scores").select("user_id,difficulty,score,streak,accuracy,distance,targets_hit,easy_clears,normal_clears,hard_clears,insane_clears,total_clears,achieved_at").eq("week_key",wk.key).eq("difficulty",diff);
  if(diff==="ENDURANCE")q=q.order("distance",{ascending:false}).order("targets_hit",{ascending:false}).order("accuracy",{ascending:false});else q=q.order("score",{ascending:false}).order("streak",{ascending:false}).order("accuracy",{ascending:false});
  const {data:rows,error}=await q.limit(20);if(error)return json({error:"ranking_failed",detail:error.message},500);
  const {pm,cm}=await enrich(admin,rows??[]);
  const entries=(rows??[]).map((r:any,idx:number)=>{const p:any=pm.get(r.user_id)||{},c:any=cm.get(r.user_id)||{};return{position:idx+1,user_id:r.user_id,display_name:p.display_name||"PLAYER",exp_rank:p.exp_rank||"",selected_name_colour:c.selected_name_colour||0,difficulty:r.difficulty,score:Number(r.score||0),streak:Number(r.streak||0),accuracy:Number(r.accuracy||0),distance:Number(r.distance||0),targets_hit:Number(r.targets_hit||0),easy_clears:Number(r.easy_clears||0),normal_clears:Number(r.normal_clears||0),hard_clears:Number(r.hard_clears||0),insane_clears:Number(r.insane_clears||0),total_clears:Number(r.total_clears||0),achieved_at:r.achieved_at}});
  return json({week_key:wk.key,starts_at:wk.start.toISOString(),resets_at:wk.end.toISOString(),timezone:"Asia/Ho_Chi_Minh",scope:"weekly",entries});
});
