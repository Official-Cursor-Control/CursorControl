import { createClient } from 'npm:@supabase/supabase-js@2'

const JSON_HEADERS = { 'content-type': 'application/json; charset=utf-8' }
const DIFFS = new Set(['OVERALL','EASY','NORMAL','HARD','INSANE','ENDURANCE','SURVIVAL'])
function json(body: unknown, status = 200) { return new Response(JSON.stringify(body), { status, headers: JSON_HEADERS }) }
function weekKey(d = new Date()) {
  const x = new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()))
  const day = x.getUTCDay() || 7
  x.setUTCDate(x.getUTCDate() + 4 - day)
  const y0 = new Date(Date.UTC(x.getUTCFullYear(), 0, 1))
  const week = Math.ceil((((x.getTime() - y0.getTime()) / 86400000) + 1) / 7)
  return `${x.getUTCFullYear()}-W${String(week).padStart(2,'0')}`
}

Deno.serve(async (req: Request) => {
  const token = (req.headers.get('authorization') ?? '').replace(/^Bearer\s+/i,'').trim()
  if (!token) return json({ error: 'unauthorized' }, 401)
  const url = Deno.env.get('SUPABASE_URL') ?? ''
  const anon = Deno.env.get('SUPABASE_ANON_KEY') ?? ''
  const service = Deno.env.get('SUPABASE_SERVICE_ROLE_KEY') ?? ''
  if (!url || !anon || !service) return json({ error: 'server_config' }, 500)
  const auth = createClient(url, anon, { auth: { persistSession: false } })
  const { data: ud, error: ue } = await auth.auth.getUser(token)
  if (ue || !ud.user) return json({ error: 'unauthorized' }, 401)
  const uid = ud.user.id
  const admin = createClient(url, service, { auth: { persistSession: false } })
  const wk = weekKey()

  if (req.method === 'POST') {
    let body: any = {}
    try { body = await req.json() } catch {}
    if (body?.action !== 'claim_weekly_reward') return json({ error: 'invalid_action' }, 400)
    const { data: own } = await admin.from('weekly_scores').select('score,streak,accuracy,distance,targets_hit,difficulty').eq('week_key',wk).eq('user_id',uid).order('score',{ascending:false}).limit(1).maybeSingle()
    if (!own) return json({ error: 'no_weekly_score' }, 404)
    const diff = String(own.difficulty)
    let q = admin.from('weekly_scores').select('user_id,score,streak,accuracy,distance,targets_hit').eq('week_key',wk).eq('difficulty',diff)
    q = diff === 'ENDURANCE' ? q.order('distance',{ascending:false}).order('targets_hit',{ascending:false}).order('accuracy',{ascending:false}) : q.order('score',{ascending:false}).order('streak',{ascending:false}).order('accuracy',{ascending:false})
    const { data: rows, error } = await q.limit(5000)
    if (error) return json({ error: 'ranking_failed' }, 500)
    const pos = (rows ?? []).findIndex((r:any)=>r.user_id===uid)+1
    if (pos < 1 || pos > 20) return json({ placement: pos, awarded: false })
    let badge='WEEKLY_FINALIST', title:string|null=null, frame:number|null=null
    if (pos===1) { badge='WEEKLY_CHAMPION'; title='WEEKLY CHAMPION'; frame=4 }
    else if (pos<=3) { badge='WEEKLY_PODIUM'; frame=4 }
    else if (pos<=10) badge='WEEKLY_TOP10'
    const expires = new Date(Date.now()+8*24*60*60*1000).toISOString()
    const { error: awardErr } = await admin.from('competitive_awards').upsert({user_id:uid,award_key:'weekly',period_key:wk,placement:pos,badge,temporary_title:title,temporary_frame:frame,expires_at:expires},{onConflict:'user_id,award_key,period_key',ignoreDuplicates:true})
    if (awardErr) return json({ error: 'award_failed' }, 500)
    return json({ week_key:wk,placement:pos,badge,temporary_title:title,temporary_frame:frame,expires_at:expires,awarded:true })
  }

  if (req.method !== 'GET') return json({ error: 'method_not_allowed' }, 405)
  const u = new URL(req.url)
  const scope = (u.searchParams.get('scope') ?? 'weekly').toLowerCase()
  const diff = (u.searchParams.get('difficulty') ?? 'OVERALL').toUpperCase()
  if (!DIFFS.has(diff)) return json({ error: 'invalid_difficulty' }, 400)
  if (scope !== 'weekly' && scope !== 'around_me') return json({ error: 'invalid_scope' }, 400)

  const table = scope === 'weekly' ? 'weekly_scores' : 'global_scores'
  let q:any = admin.from(table).select('user_id,score,streak,accuracy,distance,targets_hit,difficulty')
  if (scope === 'weekly') q=q.eq('week_key',wk)
  q=q.eq('difficulty',diff)
  q = diff === 'ENDURANCE' ? q.order('distance',{ascending:false}).order('targets_hit',{ascending:false}).order('accuracy',{ascending:false}) : q.order('score',{ascending:false}).order('streak',{ascending:false}).order('accuracy',{ascending:false})
  const { data: ranked, error } = await q.limit(scope==='weekly'?20:5000)
  if (error) return json({ error: 'ranking_failed' }, 500)
  let rows = ranked ?? []
  let start = 0
  if (scope === 'around_me') {
    const idx = rows.findIndex((r:any)=>r.user_id===uid)
    if (idx < 0) rows=[]
    else { start=Math.max(0,Math.min(idx-5,Math.max(0,rows.length-11))); rows=rows.slice(start,start+11) }
  }
  const ids=[...new Set(rows.map((r:any)=>r.user_id))]
  const { data: profiles } = ids.length ? await admin.from('player_profiles').select('user_id,display_name,selected_name_colour,exp_rank').in('user_id',ids) : {data:[] as any[]}
  const pm=new Map((profiles??[]).map((p:any)=>[p.user_id,p]))
  const entries=rows.map((r:any,i:number)=>{const p:any=pm.get(r.user_id)||{};return {position:start+i+1,user_id:r.user_id,name:p.display_name||'PLAYER',selected_name_colour:p.selected_name_colour||0,score:r.score||0,streak:r.streak||0,accuracy:r.accuracy||0,distance:r.distance||0,targets_hit:r.targets_hit||0,rank:p.exp_rank||''}})
  return json({ week_key:wk, entries })
})
