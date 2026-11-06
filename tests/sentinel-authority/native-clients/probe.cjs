// Match the inspected non-cluster factory's persistent ioredis options, then
// compare a READONLY reconnect control. No app, credentials, or user data.
const Redis = require('ioredis');
const duration = 90;
const start = Date.now();
const seconds = () => (Date.now() - start) / 1000;
const delay = (ms) => new Promise(resolve => setTimeout(resolve, ms));
const clients = {
  observed_factory: new Redis({host: 'redis-replication-master', port: 6379, maxRetriesPerRequest: null}),
  reconnect_control: new Redis({host: 'redis-replication-master', port: 6379, maxRetriesPerRequest: null,
    reconnectOnError(error) { return error.message.startsWith('READONLY'); }}),
};
const results = {};
for (const client of Object.values(clients)) client.on('error', () => {});

async function bounded(command) {
  let timer;
  try {
    return await Promise.race([command, new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error('probe-timeout')), 2000);
    })]);
  } finally { clearTimeout(timer); }
}

async function run(name, client) {
  const result = results[name] = {successes: 0, readonly: 0, timeouts: 0, other_errors: 0, last_success_seconds: 0};
  while (seconds() < duration) {
    try {
      await bounded(client.set('synthetic-native-client-' + name, 'retained-test-value'));
      result.successes++;
      result.last_success_seconds = seconds();
    } catch (error) {
      if (error.message.startsWith('READONLY')) result.readonly++;
      else if (error.message === 'probe-timeout') result.timeouts++;
      else result.other_errors++;
    }
    await delay(50);
  }
}

(async () => {
  try {
    await Promise.all(Object.values(clients).map(client => bounded(client.ping())));
    console.log(JSON.stringify({native_clients_started: true, library: 'ioredis', version: require('ioredis/package.json').version}));
    await Promise.all(Object.entries(clients).map(([name, client]) => run(name, client)));
    const control_recovered = results.reconnect_control.last_success_seconds >= 85;
    console.log(JSON.stringify({native_client_results: results, control_recovered,
      observed_factory_recovered: results.observed_factory.last_success_seconds >= 85,
      scope: 'connection factory options, not the full service or BullMQ workflow'}));
    process.exitCode = control_recovered ? 0 : 1;
  } catch (_) {
    console.log(JSON.stringify({native_client_probe_failed: true}));
    process.exitCode = 1;
  } finally {
    for (const client of Object.values(clients)) client.disconnect();
  }
})();
