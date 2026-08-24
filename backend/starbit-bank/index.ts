import { createClient } from "jsr:@supabase/supabase-js@2";

const SUPABASE_URL = Deno.env.get("SUPABASE_URL")!;
const SERVICE_ROLE = Deno.env.get("SUPABASE_SERVICE_ROLE_KEY")!;
const db = createClient(SUPABASE_URL, SERVICE_ROLE, {auth:{persistSession:false,autoRefreshToken:false}});
const H = {"Content-Type":"application/json","Cache-Control":"no-store"};
function reply(x:unknown,status=200){return new Response(JSON.stringify(x),{status,headers:H});}
function n(v:unknown){const s=String(v??"0").trim();return /^\d+$/.test(s)?s:"0";}

Deno.serve(async(req:Request)=>{
  if(req.method!=="POST") return reply({ok:false,message:"method not allowed"},405);
  const auth=req.headers.get("Authorization")||"";
  const token=auth.startsWith("Bearer ")?auth.slice(7):"";
  if(!token) return reply({ok:false,message:"Discord login required"},401);
  const {data:au,error:ae}=await db.auth.getUser(token);
  if(ae||!au.user) return reply({ok:false,message:"invalid session"},401);
  let body:any={}; try{body=await req.json();}catch{return reply({ok:false,message:"invalid json"},400);}
  const action=String(body.action||"").toLowerCase();
  const sessionToken=String(body.session_token||"").trim();
  if(!sessionToken) return reply({ok:false,message:"cloud session required"},409);
  const earned=n(body.earned_total), spent=n(body.spent_total);
  const {data,error}=await db.rpc("starbit_bank_apply",{
    p_user_id:au.user.id,
    p_session_nonce:sessionToken,
    p_action:action,
    p_earned_total:earned,
    p_spent_total:spent,
  });
  if(error){
    const m=String(error.message||"");
    if(m.includes("session_lost")||m.includes("bank_session_not_claimed")) return reply({ok:false,session_lost:true,force_logout:true,message:"Account is active on another device"},409);
    if(m.includes("non_monotonic")) return reply({ok:false,message:"Starbit session counters are invalid; reclaim the cloud session"},409);
    if(m.includes("insufficient_starbits")) return reply({ok:false,message:"Not enough banked Starbits"},400);
    return reply({ok:false,message:"Starbit Bank unavailable"},500);
  }
  const row=Array.isArray(data)&&data.length?data[0]:null;
  if(!row) return reply({ok:false,message:"Starbit Bank returned no state"},500);
  return reply({
    ok:true,
    protocol_version:2,
    bank_balance:String(row.bank_balance??"0"),
    unbanked_balance:String(row.unbanked_balance??"0"),
    total_balance:String(row.total_balance??"0"),
    revision:Number(row.revision||0),
    last_bank_at:row.last_bank_at,
    next_bank_at:row.next_bank_at,
    earned_reported:String(row.earned_reported??"0"),
    spent_reported:String(row.spent_reported??"0"),
  });
});
