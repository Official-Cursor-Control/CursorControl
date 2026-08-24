import { createClient } from "https://esm.sh/@supabase/supabase-js@2";

function json(body: Record<string, unknown>, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "content-type": "application/json; charset=utf-8",
      "cache-control": "no-store",
      "access-control-allow-origin": "*",
      "access-control-allow-headers": "authorization, apikey, content-type",
      "access-control-allow-methods": "GET, POST, OPTIONS",
    },
  });
}

function cleanIntArray(raw: unknown, min: number, max: number, allowed?: Set<number>): number[] {
  if (!Array.isArray(raw)) return [];
  return [...new Set(raw.map(Number).filter((n) => Number.isInteger(n) && n >= min && n <= max && (!allowed || allowed.has(n))))].sort((a,b)=>a-b);
}

function cleanShips(raw: unknown): number[] {
  return cleanIntArray(raw, 1, 12, new Set([1,2,3,4,5,6,7,8,9,10,12]));
}
function cleanTitles(raw: unknown): string[] {
  const out = ["ROOKIE PILOT"];
  if (Array.isArray(raw)) {
    for (const v of raw) {
      const s = String(v ?? "").trim().slice(0, 64);
      if (s && !out.includes(s)) out.push(s);
    }
  }
  return out;
}
function cleanShowcase(raw: unknown): string[] {
  const out: string[] = [];
  if (Array.isArray(raw)) {
    for (const v of raw) {
      const s = String(v ?? "").trim().slice(0, 96);
      if (s && !out.includes(s)) out.push(s);
      if (out.length >= 3) break;
    }
  }
  return out;
}
function unionNums(a: unknown, b: unknown, min: number, max: number, allowed?: Set<number>) {
  return cleanIntArray([...(Array.isArray(a)?a:[]), ...(Array.isArray(b)?b:[])], min, max, allowed);
}
function unionTitles(a: unknown, b: unknown) { return cleanTitles([...(Array.isArray(a)?a:[]), ...(Array.isArray(b)?b:[])]); }
function validSelected(n: unknown, owned: number[], max: number): number | null {
  const x = Number(n);
  if (!Number.isInteger(x) || x < 0 || x > max) return null;
  if (x === 0 || owned.includes(x)) return x;
  return null;
}
function displayName(user: any): string {
  const md = user?.user_metadata ?? {};
  const raw = md.user_name ?? md.preferred_username ?? md.global_name ?? md.name ?? md.full_name ?? md.username ?? "DISCORD PLAYER";
  return String(raw).trim().slice(0, 32) || "DISCORD PLAYER";
}
function avatarURL(user: any): string {
  const md = user?.user_metadata ?? {};
  return String(md.avatar_url ?? md.picture ?? md.avatar ?? "").trim();
}
function compareStandard(a:any,b:any):number {
  const s=Number(b?.score??0)-Number(a?.score??0); if(s) return s;
  const ac=Number(b?.accuracy??0)-Number(a?.accuracy??0); if(ac) return ac;
  const st=Number(b?.streak??0)-Number(a?.streak??0); if(st) return st;
  return String(a?.user_id??"").localeCompare(String(b?.user_id??""));
}
function compareEndurance(a:any,b:any):number {
  const s=Number(b?.score??0)-Number(a?.score??0); if(s) return s;
  const st=Number(b?.streak??0)-Number(a?.streak??0); if(st) return st;
  const ac=Number(b?.accuracy??0)-Number(a?.accuracy??0); if(ac) return ac;
  return String(a?.user_id??"").localeCompare(String(b?.user_id??""));
}

Deno.serve(async (req) => {
  if (req.method === "OPTIONS") return json({ok:true});
  const url = Deno.env.get("SUPABASE_URL") ?? "";
  const serviceKey = Deno.env.get("SUPABASE_SERVICE_ROLE_KEY") ?? "";
  if (!url || !serviceKey) return json({ok:false,error:"server is not configured"},500);
  const admin = createClient(url, serviceKey, {auth:{persistSession:false,autoRefreshToken:false}});

  if (req.method === "POST") {
    const token=(req.headers.get("authorization")??"").replace(/^Bearer\s+/i,"").trim();
    if(!token) return json({ok:false,error:"Discord login required"},401);
    const {data:authData,error:authError}=await admin.auth.getUser(token);
    if(authError||!authData.user) return json({ok:false,error:"invalid session"},401);
    let body:any; try { body=await req.json(); } catch { return json({ok:false,error:"invalid json"},400); }
    if(String(body.action??"").toLowerCase()!=="sync") return json({ok:false,error:"unknown action"},400);

    const {data:existing,error:readError}=await admin.from("global_player_profiles").select(`
      unlocked_ships,selected_ship,survival_checkpoint,
      unlocked_fire_colors,selected_fire_color,unlocked_fire_sizes,selected_fire_size,
      unlocked_titles,selected_title,unlocked_name_colours,selected_name_colour,
      unlocked_profile_frames,selected_profile_frame,unlocked_profile_skins,selected_profile_skin,achievement_showcase,
      selected_profile_font,selected_profile_name_font,selected_profile_primary_colour,selected_profile_secondary_colour,
      profile_name_shadow,profile_shadow_colour,profile_gradient_vertical,selected_profile_animation,
      sentinel_defeats,serpent_defeats,array_defeats,
      best_survival_wave,best_survival_kills,competitive_badge,season_best
    `).eq("user_id",authData.user.id).maybeSingle();
    if(readError) return json({ok:false,error:readError.message},500);
    const {data:globalProgress,error:progressError}=await admin.from("global_player_progress").select("exp,exp_rank").eq("user_id",authData.user.id).maybeSingle();
    if(progressError) return json({ok:false,error:progressError.message},500);
    const globalExp=Math.max(0,Math.trunc(Number(globalProgress?.exp??0)||0));
    const goldUnlocked=globalExp>=4000;
    const platinumUnlocked=globalExp>=12000;
    const diamondUnlocked=globalExp>=30000;
    const masterUnlocked=globalExp>=75000;

    const ships=cleanShips([...(existing?.unlocked_ships??[]), ...(body.unlocked_ships??[])]);
    const fireColors=unionNums(existing?.unlocked_fire_colors,body.unlocked_fire_colors,1,7);
    const fireSizes=unionNums(existing?.unlocked_fire_sizes,body.unlocked_fire_sizes,1,2);
    const titles=unionTitles(existing?.unlocked_titles,body.unlocked_titles);
    const nameColours=unionNums(existing?.unlocked_name_colours,body.unlocked_name_colours,1,5);
    const frames=unionNums(existing?.unlocked_profile_skins ?? existing?.unlocked_profile_frames, body.unlocked_profile_skins ?? body.unlocked_profile_frames, 101,111);

    const selectWithFallback=(incoming:unknown,cloud:unknown,owned:number[],max:number)=>{
      const local=validSelected(incoming,owned,max); if(local!==null) return local;
      const old=validSelected(cloud,owned,max); return old===null?0:old;
    };
    const selectedShip=selectWithFallback(body.selected_ship,existing?.selected_ship,ships,12);
    const selectedFireColor=selectWithFallback(body.selected_fire_color,existing?.selected_fire_color,fireColors,7);
    const selectedFireSize=selectWithFallback(body.selected_fire_size,existing?.selected_fire_size,fireSizes,2);
    const selectedNameColour=selectWithFallback(body.selected_name_colour,existing?.selected_name_colour,nameColours,5);
    // v381: restore the proven v360 banner selection contract.
    let selectedProfileFrame=selectWithFallback(body.selected_profile_skin ?? body.selected_profile_frame, existing?.selected_profile_skin ?? existing?.selected_profile_frame, frames,111);
    if(!platinumUnlocked) selectedProfileFrame=0;
    const bounded=(v:unknown,min:number,max:number,fallback:number)=>{const n=Math.trunc(Number(v));return Number.isFinite(n)&&n>=min&&n<=max?n:fallback};
    const selectedProfileFont=goldUnlocked?bounded(body.selected_profile_font,0,7,bounded(existing?.selected_profile_font,0,7,0)):0;
    const selectedProfileNameFont=diamondUnlocked?bounded(body.selected_profile_name_font,0,12,bounded(existing?.selected_profile_name_font,0,12,0)):0;
    const selectedProfilePrimaryColour=diamondUnlocked?bounded(body.selected_profile_primary_colour,0,11,bounded(existing?.selected_profile_primary_colour,0,11,1)):1;
    const selectedProfileSecondaryColour=diamondUnlocked?bounded(body.selected_profile_secondary_colour,0,11,bounded(existing?.selected_profile_secondary_colour,0,11,2)):2;
    const profileNameShadow=diamondUnlocked?(Object.prototype.hasOwnProperty.call(body,"profile_name_shadow")?Boolean(body.profile_name_shadow):Boolean(existing?.profile_name_shadow??true)):false;
    const profileShadowColour=diamondUnlocked?bounded(body.profile_shadow_colour,0,11,bounded(existing?.profile_shadow_colour,0,11,0)):0;
    const profileGradientVertical=diamondUnlocked?(Object.prototype.hasOwnProperty.call(body,"profile_gradient_vertical")?Boolean(body.profile_gradient_vertical):Boolean(existing?.profile_gradient_vertical??false)):false;
    const selectedProfileAnimation=masterUnlocked?bounded(body.selected_profile_animation,0,3,bounded(existing?.selected_profile_animation,0,3,0)):0;

    const requestedTitle=String(body.selected_title??"").trim();
    const cloudTitle=String(existing?.selected_title??"").trim();
    const selectedTitle=titles.includes(requestedTitle)?requestedTitle:(titles.includes(cloudTitle)?cloudTitle:"ROOKIE PILOT");

    const incomingCheckpoint=Math.max(1,Math.trunc(Number(body.survival_checkpoint??1)||1));
    const cloudCheckpoint=Math.max(1,Math.trunc(Number(existing?.survival_checkpoint??1)||1));
    const checkpoint=Math.max(incomingCheckpoint,cloudCheckpoint);
    const bestWave=Math.max(0,Math.trunc(Number(body.best_survival_wave??0)||0),Math.trunc(Number(existing?.best_survival_wave??0)||0));
    const bestKills=Math.max(0,Math.trunc(Number(body.best_survival_kills??0)||0),Math.trunc(Number(existing?.best_survival_kills??0)||0));
    const sentinelDefeats=Math.max(0,Math.trunc(Number(body.sentinel_defeats??0)||0),Math.trunc(Number(existing?.sentinel_defeats??0)||0));
    const serpentDefeats=Math.max(0,Math.trunc(Number(body.serpent_defeats??0)||0),Math.trunc(Number(existing?.serpent_defeats??0)||0));
    const arrayDefeats=Math.max(0,Math.trunc(Number(body.array_defeats??0)||0),Math.trunc(Number(existing?.array_defeats??0)||0));

    // The incoming showcase is an explicit ordered selection, not an unlock union.
    // If an older client omits it, preserve cloud state.
    const incomingHasShowcase=Object.prototype.hasOwnProperty.call(body,"achievement_showcase");
    const showcase=incomingHasShowcase?cleanShowcase(body.achievement_showcase):cleanShowcase(existing?.achievement_showcase);

    const row:any={
      user_id:authData.user.id,
      unlocked_ships:ships,selected_ship:selectedShip,survival_checkpoint:checkpoint,
      unlocked_fire_colors:fireColors,selected_fire_color:selectedFireColor,
      unlocked_fire_sizes:fireSizes,selected_fire_size:selectedFireSize,
      unlocked_titles:titles,selected_title:selectedTitle,
      unlocked_name_colours:nameColours,selected_name_colour:selectedNameColour,
      unlocked_profile_frames:frames,selected_profile_frame:selectedProfileFrame,
      unlocked_profile_skins:frames,selected_profile_skin:selectedProfileFrame,
      selected_profile_font:selectedProfileFont,
      selected_profile_name_font:selectedProfileNameFont,
      selected_profile_primary_colour:selectedProfilePrimaryColour,
      selected_profile_secondary_colour:selectedProfileSecondaryColour,
      profile_name_shadow:profileNameShadow,
      profile_shadow_colour:profileShadowColour,
      profile_gradient_vertical:profileGradientVertical,
      selected_profile_animation:selectedProfileAnimation,
      sentinel_defeats:sentinelDefeats,serpent_defeats:serpentDefeats,array_defeats:arrayDefeats,
      achievement_showcase:showcase,best_survival_wave:bestWave,best_survival_kills:bestKills,
      updated_at:new Date().toISOString(),
    };
    const {error}=await admin.from("global_player_profiles").upsert(row,{onConflict:"user_id"});
    if(error) return json({ok:false,error:error.message},500);

    return json({
      ok:true,
      unlocked_ships:ships,selected_ship:selectedShip,survival_checkpoint:checkpoint,
      unlocked_fire_colors:fireColors,selected_fire_color:selectedFireColor,
      unlocked_fire_sizes:fireSizes,selected_fire_size:selectedFireSize,
      unlocked_titles:titles,selected_title:selectedTitle,
      unlocked_name_colours:nameColours,selected_name_colour:selectedNameColour,
      unlocked_profile_frames:frames,selected_profile_frame:selectedProfileFrame,
      unlocked_profile_skins:frames,selected_profile_skin:selectedProfileFrame,
      selected_profile_font:selectedProfileFont,
      selected_profile_name_font:selectedProfileNameFont,
      selected_profile_primary_colour:selectedProfilePrimaryColour,
      selected_profile_secondary_colour:selectedProfileSecondaryColour,
      profile_name_shadow:profileNameShadow,
      profile_shadow_colour:profileShadowColour,
      profile_gradient_vertical:profileGradientVertical,
      selected_profile_animation:selectedProfileAnimation,
      sentinel_defeats:sentinelDefeats,serpent_defeats:serpentDefeats,array_defeats:arrayDefeats,
      achievement_showcase:showcase,best_survival_wave:bestWave,best_survival_kills:bestKills,
      competitive_badge:String(existing?.competitive_badge??""),season_best:String(existing?.season_best??""),
    });
  }

  if(req.method!=="GET") return json({ok:false,error:"method not allowed"},405);
  const u=new URL(req.url);
  let userID=(u.searchParams.get("user_id")??"").trim();
  const name=(u.searchParams.get("name")??"").trim();
  if(!userID&&name){
    const {data:rows}=await admin.from("global_player_progress").select("user_id,display_name").ilike("display_name",name).limit(1);
    userID=String(rows?.[0]?.user_id??"");
    if(!userID){
      const {data:rows2}=await admin.from("global_scores").select("user_id,display_name").ilike("display_name",name).limit(1);
      userID=String(rows2?.[0]?.user_id??"");
    }
  }
  if(!userID) return json({ok:false,error:"player not found"},404);

  const [authResult,progressResult,scoresResult,profileResult]=await Promise.all([
    admin.auth.admin.getUserById(userID),
    admin.from("global_player_progress").select(`user_id,display_name,exp,exp_rank,easy_clears,normal_clears,hard_clears,insane_clears,total_clears,updated_at`).eq("user_id",userID).maybeSingle(),
    admin.from("global_scores").select(`user_id,display_name,difficulty,score,streak,accuracy,target_count,run_time_ms,exp,exp_rank,achieved_at`).eq("user_id",userID),
    admin.from("global_player_profiles").select(`unlocked_ships,selected_ship,survival_checkpoint,unlocked_fire_colors,selected_fire_color,unlocked_fire_sizes,selected_fire_size,unlocked_titles,selected_title,unlocked_name_colours,selected_name_colour,unlocked_profile_frames,selected_profile_frame,unlocked_profile_skins,selected_profile_skin,achievement_showcase,selected_profile_font,selected_profile_name_font,selected_profile_primary_colour,selected_profile_secondary_colour,profile_name_shadow,profile_shadow_colour,profile_gradient_vertical,selected_profile_animation,sentinel_defeats,serpent_defeats,array_defeats,best_survival_wave,best_survival_kills,competitive_badge,season_best,updated_at`).eq("user_id",userID).maybeSingle(),
  ]);
  const user=authResult.data?.user;
  const progress:any=progressResult.data??{};
  const cosmetic:any=profileResult.data??{};

  const [{data:allProgress},{data:allScores}]=await Promise.all([
    admin.from("global_player_progress").select(`user_id,total_clears,exp`).limit(10000),
    admin.from("global_scores").select(`user_id,difficulty,score,streak,accuracy`).limit(50000),
  ]);
  const positions:Record<string,number>={OVERALL:0,EASY:0,NORMAL:0,HARD:0,INSANE:0,ENDURANCE:0};
  const progressRows=[...(allProgress??[])].sort((a:any,b:any)=>Number(b.total_clears??0)-Number(a.total_clears??0)||Number(b.exp??0)-Number(a.exp??0)||String(a.user_id??"").localeCompare(String(b.user_id??"")));
  const oi=progressRows.findIndex((r:any)=>String(r.user_id)===userID); if(oi>=0) positions.OVERALL=oi+1;
  for(const d of ["EASY","NORMAL","HARD","INSANE","ENDURANCE"]){
    const rows=(allScores??[]).filter((r:any)=>String(r.difficulty??"").toUpperCase()===d).sort(d==="ENDURANCE"?compareEndurance:compareStandard);
    const i=rows.findIndex((r:any)=>String(r.user_id)===userID); if(i>=0) positions[d]=i+1;
  }
  const easy=Number(progress.easy_clears??0),normal=Number(progress.normal_clears??0),hard=Number(progress.hard_clears??0),insane=Number(progress.insane_clears??0);
  return json({ok:true,profile:{
    user_id:userID,
    display_name:String(progress.display_name??(user?displayName(user):name)??"PLAYER"),
    avatar_url:user?avatarURL(user):"",created_at:String(user?.created_at??""),
    exp:Number(progress.exp??0),exp_rank:String(progress.exp_rank??"BRONZE I"),
    easy_clears:easy,normal_clears:normal,hard_clears:hard,insane_clears:insane,total_clears:Number(progress.total_clears??(easy+normal+hard+insane)),
    unlocked_ships:Array.isArray(cosmetic.unlocked_ships)?cosmetic.unlocked_ships:[],selected_ship:Number(cosmetic.selected_ship??0),
    unlocked_titles:Array.isArray(cosmetic.unlocked_titles)?cosmetic.unlocked_titles:["ROOKIE PILOT"],selected_title:String(cosmetic.selected_title??"ROOKIE PILOT"),
    unlocked_name_colours:Array.isArray(cosmetic.unlocked_name_colours)?cosmetic.unlocked_name_colours:[],selected_name_colour:Number(cosmetic.selected_name_colour??0),
    unlocked_profile_frames:Array.isArray(cosmetic.unlocked_profile_skins)?cosmetic.unlocked_profile_skins:(Array.isArray(cosmetic.unlocked_profile_frames)?cosmetic.unlocked_profile_frames:[]),selected_profile_frame:Number(cosmetic.selected_profile_skin??cosmetic.selected_profile_frame??0),
    unlocked_profile_skins:Array.isArray(cosmetic.unlocked_profile_skins)?cosmetic.unlocked_profile_skins:(Array.isArray(cosmetic.unlocked_profile_frames)?cosmetic.unlocked_profile_frames:[]),selected_profile_skin:Number(cosmetic.selected_profile_skin??cosmetic.selected_profile_frame??0),
    selected_profile_font:Number(cosmetic.selected_profile_font??0),selected_profile_name_font:Number(cosmetic.selected_profile_name_font??0),selected_profile_primary_colour:Number(cosmetic.selected_profile_primary_colour??1),selected_profile_secondary_colour:Number(cosmetic.selected_profile_secondary_colour??2),profile_name_shadow:Boolean(cosmetic.profile_name_shadow??true),profile_shadow_colour:Number(cosmetic.profile_shadow_colour??0),profile_gradient_vertical:Boolean(cosmetic.profile_gradient_vertical??false),selected_profile_animation:Number(cosmetic.selected_profile_animation??0),
    sentinel_defeats:Number(cosmetic.sentinel_defeats??0),serpent_defeats:Number(cosmetic.serpent_defeats??0),array_defeats:Number(cosmetic.array_defeats??0),
    competitive_badge:String(cosmetic.competitive_badge??""),season_best:String(cosmetic.season_best??""),
    achievement_showcase:cleanShowcase(cosmetic.achievement_showcase),best_survival_wave:Number(cosmetic.best_survival_wave??0),best_survival_kills:Number(cosmetic.best_survival_kills??0),
    survival_checkpoint:Number(cosmetic.survival_checkpoint??1),positions,scores:scoresResult.data??[],
  }});
});
