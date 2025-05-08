import logging
import traceback
from typing import Optional
from urllib.parse import urljoin

import asyncio
import httpx


async def validate_git_url(client: httpx.AsyncClient, url: Optional[str]):
    if not isinstance(url, str):
        raise ValueError('Invalid URL')
    if not url.startswith('https://') and not url.startswith('http://'):
        return False

    if not url.endswith('/'):
        url += '/'

    params = {
        'service': 'git-upload-pack',
    }
    headers = {
        'User-Agent': 'code/0.1.0',
        'Git-Protocol': 'version=2',
    }
    refs_path = 'info/refs'
    refs_url = urljoin(url, refs_path)
    logging.info('GET %s', refs_url)
    r = None
    for _ in range(5):
        try:
            r = await client.get(refs_url, params=params, headers=headers, follow_redirects=True)
            break
        except Exception:
            traceback.print_exc()
            print('retrying...')
            await asyncio.sleep(3)
    if r is None:
        return False
    if r.headers.get('Content-Type') != 'application/x-git-upload-pack-advertisement':
        # raise ValueError(f'Invalid Content-Type: {r.headers.get("Content-Type")}')
        return False
    
    return True
