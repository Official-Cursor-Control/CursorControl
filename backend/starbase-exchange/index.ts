import { createClient } from "jsr:@supabase/supabase-js@2";

const SUPABASE_URL = Deno.env.get("SUPABASE_URL")!;
const SERVICE_ROLE = Deno.env.get("SUPABASE_SERVICE_ROLE_KEY")!;
const db = createClient(SUPABASE_URL, SERVICE_ROLE, { auth: { persistSession: false, autoRefreshToken: false } });

const H = { "Content-Type": "application/json", "Cache-Control": "no-store" };
const SPACECOIN = [
  { cost: 100_000_000n, reward: 50 },
  { cost: 500_000_000n, reward: 260 },
  { cost: 1_000_000_000n, reward: 550 },
  { cost: 5_000_000_000n, reward: 3000 },
] as const;
const EXP = [
  { cost: 100_000_000n, reward: 10 },
  { cost: 500_000_000n, reward: 55 },
  { cost: 1_000_000_000n, reward: 120 },
  { cost: 5_000_000_000n, reward: 650 },
] as const;
const RANKS = [
  ["BRONZE I",0],["BRONZE II",100],["BRONZE III",300],["SILVER I",750],["SILVER II",1500],["SILVER III",2500],
  ["GOLD I",4000],["GOLD II",6000],["GOLD III",8500],["PLATINUM I",12000],["PLATINUM II",16000],["PLATINUM III",22000],
  ["DIAMOND I",30000],["DIAMOND II",40000],["DIAMOND III",55000],["MASTER I",75000],["MASTER II",100000],["MASTER III",135000]
] as const;
function reply(x: unknown, status=200){ return new Response(JSON.stringify(x), { status, headers: H }); }
function rankForExp(exp:number){ let rank="BRONZE I"; for(const [name,threshold] of RANKS){ if(exp>=threshold) rank=name; else break; } return rank; }
function displayName(user:any){ const m=user?.user_metadata??{}; return String(m.user_name??m.preferred_username??m.global_name??m.name??m.full_name??m.username??"PLAYER").trim().slice(0,32)||"PLAYER"; }
function toBigInt(v: unknown){ try { return BigInt(String(v ?? "0")); } catch { return 0n; } }

Deno.serve(async (req:Request) => {
  if(req.method !== "POST") return reply({ok:false,message:"method not allowed"},405);
  const auth = req.headers.get("Authorization") || "";
  const token = auth.startsWith("Bearer ") ? auth.slice(7) : "";
  if(!token) return reply({ok:false,message:"Discord login required"},401);
  const {data:au,error:ae} = await db.auth.getUser(token);
  if(ae || !au.user) return reply({ok:false,message:"invalid session"},401);
  let body:any={}; try { body = await req.json(); } catch { return reply({ok:false,message:"invalid json"},400); }
  const kind = String(body.kind||"").toLowerCase();
  const bundle = Math.trunc(Number(body.bundle));
  const deviceId = String(body.device_id||"").trim();
  const sessionToken = String(body.session_token||"").trim();
  if(!deviceId || !sessionToken) return reply({ok:false,message:"Starbase cloud session required"},409);
  const {data:session,error:se} = await db.from("active_game_sessions").select("device_id,session_nonce").eq("user_id",au.user.id).maybeSingle();
  if(se) return reply({ok:false,message:"session lookup failed"},500);
  if(!session || session.device_id !== deviceId || String(session.session_nonce) !== sessionToken) return reply({ok:false,message:"Account is active on another device"},409);
  const table = kind === "spacecoins" ? SPACECOIN : kind === "exp" ? EXP : null;
  if(!table || bundle < 0 || bundle >= table.length) return reply({ok:false,message:"invalid exchange bundle"},400);
  const offer = table[bundle];
  const {data:progress,error:pe} = await db.from("afk_starbit_progress").select("starbits,revision").eq("user_id",au.user.id).maybeSingle();
  if(pe || !progress) return reply({ok:false,message:"Starbase cloud progress unavailable"},409);
  const balance = toBigInt(progress.starbits);
  if(balance < offer.cost) return reply({ok:false,message:"not enough Starbits"},400);
  const remaining = balance - offer.cost;
  const currentRevision = Number(progress.revision||1);
  const {data:spent,error:spendErr} = await db.from("afk_starbit_progress").update({starbits:remaining.toString(),revision:currentRevision+1,updated_at:new Date().toISOString()}).eq("user_id",au.user.id).eq("revision",currentRevision).select("starbits,revision").maybeSingle();
  if(spendErr || !spent) return reply({ok:false,message:"balance changed; try again"},409);
  if(kind === "spacecoins") return reply({ok:true,starbits:String(spent.starbits),spacecoins_reward:offer.reward,exp_reward:0,global_exp:0,global_rank:""});
  const {data:gp,error:gpe} = await db.from("global_player_progress").select("exp").eq("user_id",au.user.id).maybeSingle();
  if(gpe){ await db.from("afk_starbit_progress").update({starbits:balance.toString(),revision:currentRevision+2,updated_at:new Date().toISOString()}).eq("user_id",au.user.id).eq("revision",currentRevision+1); return reply({ok:false,message:"Global EXP unavailable; Starbits refunded"},500); }
  const oldExp = Math.max(0, Math.trunc(Number(gp?.exp??0)||0));
  const newExp = Math.min(100_000_000, oldExp + offer.reward);
  const rank = rankForExp(newExp);
  let expErr:any = null;
  if(gp){ const r = await db.from("global_player_progress").update({exp:newExp,exp_rank:rank,updated_at:new Date().toISOString()}).eq("user_id",au.user.id); expErr = r.error; }
  else { const r = await db.from("global_player_progress").insert({user_id:au.user.id,display_name:displayName(au.user),exp:newExp,exp_rank:rank,easy_clears:0,normal_clears:0,hard_clears:0,insane_clears:0,updated_at:new Date().toISOString()}); expErr = r.error; }
  if(expErr){ await db.from("afk_starbit_progress").update({starbits:balance.toString(),revision:currentRevision+2,updated_at:new Date().toISOString()}).eq("user_id",au.user.id).eq("revision",currentRevision+1); return reply({ok:false,message:"Global EXP grant failed; Starbits refunded"},500); }
  return reply({ok:true,starbits:String(spent.starbits),spacecoins_reward:0,exp_reward:offer.reward,global_exp:newExp,global_rank:rank});
});
