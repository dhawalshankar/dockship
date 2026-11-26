const express = require('express');
const app = express();
const port = process.env.PORT || 3000;

app.get('/', (req, res) => {
  res.json({ status: 'ok', service: 'sample-node-api', uptime: process.uptime() });
});

app.get('/health', (req, res) => {
  res.json({ healthy: true });
});

app.listen(port, () => {
  console.log(`Server listening on port ${port}`);
});
