import { createClient } from 'npm:@supabase/supabase-js@2'

const JSON_HEADERS = { 'content-type': 'application/json; charset=utf-8' }
const ALLOWED_EVENTS = new Set([
  'session_started','mode_selected','run_started','run_completed','run_failed',
  'achievement_unlocked','space_cache_open_started','space_cache_ship','boss_attempted',
  'boss_cleared','tutorial_completed','profile_customized','analytics_consent_changed',
])
const BLOCKED_KEYS = /(name|email|discord|cursor|mouse|chat|message|ip|location|address)/i

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: JSON_HEADERS })
}

Deno.serve(async (req: Request) => {
  if (req.method !== 'POST') return json({ error: 'method_not_allowed' }, 405)
  const authHeader = req.headers.get('authorization') ?? ''
  const token = authHeader.replace(/^Bearer\s+/i, '').trim()
  if (!token) return json({ error: 'unauthorized' }, 401)

  const url = Deno.env.get('SUPABASE_URL') ?? ''
  const anon = Deno.env.get('SUPABASE_ANON_KEY') ?? ''
  const service = Deno.env.get('SUPABASE_SERVICE_ROLE_KEY') ?? ''
  if (!url || !anon || !service) return json({ error: 'server_config' }, 500)

  const auth = createClient(url, anon, { auth: { persistSession: false } })
  const { data: userData, error: userErr } = await auth.auth.getUser(token)
  if (userErr || !userData.user) return json({ error: 'unauthorized' }, 401)
  const userId = userData.user.id

  const raw = await req.text()
  if (raw.length > 128_000) return json({ error: 'payload_too_large' }, 413)
  let parsed: any
  try { parsed = JSON.parse(raw) } catch { return json({ error: 'invalid_json' }, 400) }
  const events = Array.isArray(parsed?.events) ? parsed.events : []
  if (events.length < 1 || events.length > 100) return json({ error: 'invalid_event_count' }, 400)

  const rows = [] as any[]
  for (const event of events) {
    const eventName = String(event?.event ?? '').trim()
    if (!ALLOWED_EVENTS.has(eventName)) continue
    const mode = String(event?.mode ?? '').toUpperCase().slice(0, 16)
    const safeFields: Record<string, string|number|boolean|null> = {}
    const fields = event?.fields && typeof event.fields === 'object' && !Array.isArray(event.fields) ? event.fields : {}
    let count = 0
    for (const [key, value] of Object.entries(fields)) {
      if (++count > 24 || BLOCKED_KEYS.test(key) || key.length > 40) continue
      if (value === null || ['string','number','boolean'].includes(typeof value)) {
        safeFields[key] = typeof value === 'string' ? value.slice(0, 96) : value as any
      }
    }
    const clientAt = typeof event?.at === 'string' && !Number.isNaN(Date.parse(event.at)) ? event.at : null
    rows.push({ user_id: userId, client_at: clientAt, event_name: eventName, mode: mode || null, fields: safeFields })
  }
  if (!rows.length) return json({ accepted: 0 })

  const admin = createClient(url, service, { auth: { persistSession: false } })
  const { error } = await admin.from('gameplay_analytics_events').insert(rows)
  if (error) return json({ error: 'storage_failed' }, 500)
  return json({ accepted: rows.length })
})
