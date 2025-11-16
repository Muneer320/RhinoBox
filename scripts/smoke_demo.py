"""Verify the local Compose demo using synthetic records and disposable files."""
import argparse
import datetime
import json
from pathlib import Path
import subprocess
import time
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urlparse
from urllib.request import Request, urlopen
import uuid


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base-url', default='http://127.0.0.1:8090')
    parser.add_argument('--project', default='rhinobox-demo')
    parser.add_argument('--state-out', type=Path)
    parser.add_argument('--verify-state', type=Path)
    args = parser.parse_args()
    assert urlparse(args.base_url).hostname in {'127.0.0.1', 'localhost'}, 'Use a local demo endpoint.'
    base = args.base_url.rstrip('/')
    repo = Path(__file__).resolve().parents[1]

    def http(path, body=None, method=None, status=200, content_type='application/json'):
        payload = json.dumps(body).encode() if body is not None and not isinstance(body, bytes) else body
        request = Request(base + path, data=payload, method=method, headers={'Content-Type': content_type})
        try:
            response = urlopen(request, timeout=30)
        except HTTPError as error:
            response = error
        with response:
            data = response.read()
            assert response.status == status, (path, response.status, data.decode(errors='replace'))
            headers = response.headers
        return data, headers

    def api(path, body=None, method=None, status=200):
        return json.loads(http(path, body, method, status)[0])

    def command(service, *parts):
        result = subprocess.run(['docker', 'compose', '-p', args.project, 'exec', '-T', service, *parts], cwd=repo, text=True, capture_output=True, check=True)
        return result.stdout.strip()

    def sql(query):
        return command('postgres', 'psql', '-U', 'rhinobox', '-d', 'rhinobox', '-Atc', query)

    def mongo(js):
        return command('mongodb', 'mongosh', '--quiet', 'rhinobox', '--eval', js)

    def verify(state):
        for table, expected in state['sql_tables'].items():
            assert table.replace('_', '').isalnum()
            assert int(sql(f'SELECT count(*) FROM "{table}"')) == expected
        collection = state['mongo_collection']
        count = mongo('print(db.getCollection(' + json.dumps(collection) + ').countDocuments({_id:{$in:' + json.dumps(state['mongo_ids']) + '}}))')
        assert int(count) == len(state['mongo_ids']), count
        value = mongo('print(db.getCollection(' + json.dumps(collection) + ').findOne({_id:' + json.dumps(state['mongo_ids'][0]) + '}).big.toString())')
        assert value == '9007199254740993', value
        files = api('/files/type/json')['files']
        hashes = {item['hash'] for item in files}
        for file_hash in state.get('archive_hashes', []):
            assert file_hash in hashes, 'Retained archive missing from the file index.'
            assert http('/files/download?' + urlencode({'hash': file_hash}))[0].strip()

    for attempt in range(30):
        try:
            api('/healthz')
            break
        except (URLError, OSError, AssertionError):
            if attempt == 29:
                raise
            time.sleep(1)

    if args.verify_state:
        verify(json.loads(args.verify_state.read_text(encoding='utf-8')))
        print('Database records and indexed archives survived the service restart.')
        return

    config = api('/config')
    assert not config['auth_enabled'] and not config['features']['multi_tenant']
    prefix = 'demo_' + uuid.uuid4().hex[:12]
    docs = [{'id': 1, 'Customer Name': 'Ada', 'quote"key': 'x', 'created_at': '2025-11-16', 'amount': 12.5}, {'id': 2, 'Customer Name': 'Lin', 'quote"key': 'y', 'created_at': '2025-11-16', 'amount': 7.5}]
    first = api('/ingest/json', {'namespace': prefix + '_sql', 'documents': docs})
    assert first['storage_backend'] == 'postgres' and first['schema_created']
    assert first['records_inserted'] == first['records_archived'] == 2
    table = first['decision']['table']
    again = api('/ingest/json', {'namespace': prefix + '_sql', 'documents': docs})
    assert again['decision']['table'] == table and not again['schema_created']
    evolved = api('/ingest/json', {'namespace': prefix + '_sql', 'documents': [{'id': 3, 'active': True}]})
    assert evolved['decision']['table'] != table and evolved['schema_created']
    assert sql(f'SELECT count(*), sum(amount) FROM "{table}"') == '4|40'
    assert sql(f'SELECT "Customer Name" FROM "{table}" WHERE id=1 LIMIT 1') == 'Ada'
    bad_namespace = prefix + '_rollback'
    error = api('/ingest/json', {'namespace': bad_namespace, 'document': {'overflow': 9223372036854775808}}, status=500)
    assert error['error']['code'] == 'INTERNAL_SERVER_ERROR'
    assert sql("SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tablename LIKE '" + bad_namespace + "%'") == '0'

    def nested(key):
        return {'_id': key, 'big': 9007199254740993, 'user': {'profile': {'preferences': {'theme': 'dark'}}}}
    ids = [prefix + suffix for suffix in ('_one', '_two', '_three')]
    events = api('/ingest/json', {'namespace': prefix + '_events', 'documents': [nested(ids[0]), nested(ids[1])]})
    assert events['storage_backend'] == 'mongodb' and events['records_inserted'] == 2
    partial = api('/ingest/json', {'namespace': prefix + '_events', 'documents': [nested(ids[0]), nested(ids[2])]})
    assert partial['records_inserted'] == 1 and partial['records_archived'] == 2 and partial['warnings']
    state = {'sql_tables': {table: 4, evolved['decision']['table']: 1}, 'mongo_collection': events['decision']['table'], 'mongo_ids': ids,
             'archive_hashes': [evolved['archive_hash'], events['archive_hash'], partial['archive_hash']]}
    verify(state)

    files = api('/files/type/json')['files']
    assert any(item['hash'] == first['archive_hash'] for item in files)
    archive, headers = http('/files/download?' + urlencode({'hash': first['archive_hash']}))
    assert len(archive.splitlines()) == 2 and headers.get('Content-Security-Policy')
    api('/files/rename', {'hash': first['archive_hash'], 'new_name': prefix + '.ndjson'}, 'PATCH')
    api('/files/' + first['archive_hash'], method='DELETE')
    assert sql(f'SELECT count(*) FROM "{table}"') == '4', 'Deleting an archive must not delete database records.'

    boundary = uuid.uuid4().hex
    text = 'Synthetic smoke file ' + prefix + ' needle'
    multipart = (f'--{boundary}\r\nContent-Disposition: form-data; name="files"; filename="{prefix}.txt"\r\nContent-Type: text/plain\r\n\r\n{text}\r\n--{boundary}--\r\n').encode()
    uploaded = json.loads(http('/ingest', multipart, content_type='multipart/form-data; boundary=' + boundary)[0])
    file_hash = uploaded['results']['files'][0]['hash']
    assert http('/files/download?hash=' + file_hash)[0].decode() == text
    result = api('/files/search?' + urlencode({'name': prefix, 'content_search': 'needle', 'date_to': datetime.datetime.now(datetime.timezone.utc).date().isoformat()}))
    assert result['count'] == 1
    api('/files/' + file_hash + '/move', {'new_category': 'documents/demo'}, 'PATCH')
    note = api('/files/' + file_hash + '/notes', {'text': 'Synthetic demo note', 'author': 'demo'}, status=201)
    api('/files/' + file_hash + '/notes/' + note['id'], {'text': 'Updated synthetic note'}, 'PATCH')
    assert api('/files/' + file_hash + '/notes')['notes'][0]['text'] == 'Updated synthetic note'
    api('/files/' + file_hash, method='DELETE')
    api('/files/' + file_hash, status=404)
    if args.state_out:
        args.state_out.write_text(json.dumps(state, indent=2) + '\n', encoding='utf-8')
    print('Demo smoke passed: SQL inserts/schema reuse/evolution/rollback; MongoDB int64/partial writes; indexed JSON archives; upload/search/download/move/notes/delete.')
    print('Synthetic database records and remaining archives were retained for the persistence check.')


if __name__ == '__main__':
    main()
