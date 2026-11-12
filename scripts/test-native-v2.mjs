#!/usr/bin/env node
import { spawn } from 'node:child_process'
import { createInterface } from 'node:readline'
import { fileURLToPath } from 'node:url'

if (!process.env.E2B_TEST_OWNER_DSN) throw new Error('E2B_TEST_OWNER_DSN must name the owned PostgreSQL fixture')
const backend = fileURLToPath(new URL('../backend/', import.meta.url))
const required = ['TestTLSCallbackOriginalTokenAndOneUse', 'TestPrivateFirstByteAllPaths',
  'TestDurableCallbackCommitWithheldNeverEmitsOrReplays', 'TestV2ReadActualSessionLosslessWireAndChangedCapture',
  'TestV2DisclosureActualSessionCheckedWireAndChangedCapture', 'TestV2WalletCaptureChangesBeforeAndAfterForward']
const selected = process.argv[2] === 'prepare' ? ['TestPrepareOwnedV2Database'] : required
if (process.argv.length > 3 || process.argv[2] && process.argv[2] !== 'prepare') throw new Error('Usage: node scripts/test-native-v2.mjs [prepare]')
const child = spawn(process.env.GO ?? 'go', ['test', '-json', '-p', '1', '-tags', 'privacy', '-count=1', '-timeout', '5m',
  '-run', `^(${selected.join('|')})$`, './internal/authv2', ...(process.argv[2] === 'prepare' ? [] : ['./internal/api'])],
{ cwd: backend, env: { ...process.env, E2B_TEST_INITIALIZE: process.argv[2] === 'prepare' ? '1' : '0' }, stdio: ['ignore', 'pipe', 'inherit'] })
const passed = new Set(), skipped = new Set(), output = []
let failed = false
const reading = (async () => {
  for await (const line of createInterface({ input: child.stdout, crlfDelay: Infinity })) {
    try {
      const event = JSON.parse(line)
      if (event.Output) output.push(event.Output)
      if (event.Action === 'pass' && event.Test) { passed.add(event.Test); if (selected.includes(event.Test)) console.log('PASS', event.Test) }
      if (event.Action === 'skip') skipped.add(event.Test)
      if (event.Action === 'fail') failed = true
    } catch { failed = true; process.stderr.write(line + '\n') }
  }
})()
const code = await new Promise((resolve, reject) => { child.once('error', reject); child.once('exit', resolve) })
await reading
const missing = selected.filter(name => !passed.has(name) || skipped.has(name))
if (code !== 0 || failed || missing.length) {
  process.stderr.write(output.join('').slice(-16_384))
  throw new Error(`Required native Explorer controls failed or skipped: ${missing.join(', ') || 'see package failure'}`)
}
console.log(`PASS ${selected.length} required native Explorer controls executed`)
