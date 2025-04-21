box.cfg{ listen = 3301 }

-- space для сохранения запросов / ответов
box.schema.sequence.create('req_seq', { if_not_exists = true })

local s = box.schema.space.create('requests', { if_not_exists = true })
s:format({
  { name = 'id',     type = 'unsigned' },
  { name = 'host',   type = 'string'   },
  { name = 'method', type = 'string'   },
  { name = 'path',   type = 'string'   },
  { name = 'data',   type = 'map'      },
  { name = 'ts',     type = 'unsigned' },
})
s:create_index('primary', { parts = { 'id' }, sequence = 'req_seq', if_not_exists = true })
