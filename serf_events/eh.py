#!/usr/bin/env python
from serf_master import SerfHandler, SerfHandlerProxy
import sys
import os
import json
import subprocess

import logging
logging.basicConfig(filename='eh.log', level=logging.DEBUG)


class DefaultHandler(SerfHandler):
    def __init__(self):
        super(DefaultHandler, self).__init__()
        self.logger = logging.getLogger(self.name)

        self.member_list = []
        for line in sys.stdin:
            m = line.split('\t')
            if len(m) >= 4:
                tags = m[3].split(',')
                t = {}
                for tag in tags:
                    (k, v) = tuple(tag.split('='))
                    t[k.strip()] = v.strip()
                self.member_list.append("{}:{}".format(t['rphost'], t['rpport']))

        self.rphost = os.environ.get('SERF_TAG_RPHOST', 'rphost')
        self.rpport = os.environ.get('SERF_TAG_RPPORT', 'rpport')
        self.rphostport = "{}:{}".format(self.rphost, self.rpport)

    def deploy(self):
        self.logger.info('deploy')

    def emit(self, name):
        ret = subprocess.call(['tcurl', '-p', self.rphostport, 'ringpop',
                               '/hashring/{}'.format(name), json.dumps({'members': self.member_list})])
        self.logger.info("{} {} {} {}"
                         .format(name, self.member_list, self.rphostport, ret))

    def member_join(self):
        self.emit('add')

    def member_leave(self):
        self.emit('remove')

    def member_failed(self):
        self.emit('remove')


if __name__ == '__main__':
    try:
        handler = SerfHandlerProxy()
        handler.register('default', DefaultHandler())
        handler.run()
    except Exception as e:
        logging.getLogger("main").exception(e)
