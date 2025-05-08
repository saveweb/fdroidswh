import asyncio
import logging
import time
import traceback

import httpx

from fdroid_push_swh.git import validate_git_url

async def post_git_url(client: httpx.AsyncClient, url: str, swh_token: str):
    #   POST https://archive.softwareheritage.org/api/1/origin/save/git/url/https://github.com/${GITHUB_REPOSITORY}/
    if not url.endswith('/'):
        url += '/'
    headers = {
        'Authorization': f'Bearer {swh_token}',
        }
    e = 0
    r=None
    while True:
        try:
            r = await client.post(f'https://archive.softwareheritage.org/api/1/origin/save/git/url/{url}', headers=headers, follow_redirects=True)
        except Exception:
            e += 1
            if e > 10:
                return
            await asyncio.sleep(3)
        assert r
        logging.info('X-RateLimit-Remaining: %s', r.headers.get('X-RateLimit-Remaining'))
        if r.status_code == 429:
            waiting_to = int(r.headers.get("x-ratelimit-reset", time.time())) - time.time() + 10
            logging.warning(f'Hitting rate limit. (sleep {waiting_to}s)')
            await asyncio.sleep(waiting_to)
            continue
        break
    if r.status_code != 200:
        if r.status_code == 429:
            logging.warning(f'Hitting rate limit: {r.headers}')
            raise ValueError(f'429 Too Many Requests: {r.text}')
        raise ValueError(f'Invalid status code: {r.status_code}')
    if r.headers.get('Content-Type') != 'application/json':
        raise ValueError(f'Invalid Content-Type: {r.headers.get("Content-Type")}')
    r_json = r.json()
    save_task_status = r_json['save_task_status']
    save_request_status = r_json['save_request_status']
    request_url = r_json['request_url']
    return

    # Why I commented out the following code? IDK what I thought 1 year ago (2023), lol.
    while True:
        await asyncio.sleep(10)
        r = await client.get(request_url, headers=headers, follow_redirects=True)
        logging.info('X-RateLimit-Remaining: %s', r.headers.get('X-RateLimit-Remaining'))
        if r.status_code != 200:
            raise ValueError(f'Invalid status code: {r.status_code}')
        if r.headers.get('Content-Type') != 'application/json':
            raise ValueError(f'Invalid Content-Type: {r.headers.get("Content-Type")}')
        r_json = r.json()
        save_request_status = r_json['save_request_status']
        save_task_status = r_json['save_task_status']
        if save_task_status in ['succeeded', 'failed']:
            logging.info('save_task_status: %s %s', save_task_status, save_request_status)
            break
        logging.info('save_task_status: %s %s', save_task_status, save_request_status)

async def git_swh(git_url: str, swh_token: str):
    async with httpx.AsyncClient() as client:
        is_valid_repo = await validate_git_url(client=client, url=git_url)
        if not is_valid_repo:
            logging.warning('Invalid git repository')
            return 'Invalid git repository'

        try:
            await post_git_url(client=client, url=git_url, swh_token=swh_token)
        except Exception:
            traceback.print_exc()
            return