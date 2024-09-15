import logging
from fdroid_push_swh.main import main as lib_main

import asyncio

def main():
    logging.basicConfig(level=logging.INFO)
    asyncio.run(lib_main())