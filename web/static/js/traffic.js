// MikVoc traffic.js — Live traffic chart using Chart.js + SSE polling

document.addEventListener('DOMContentLoaded', () => {
  const canvas = document.getElementById('traffic-chart');
  if (!canvas) return;

  const MAX_POINTS = 30;
  const rxData = new Array(MAX_POINTS).fill(0);
  const txData = new Array(MAX_POINTS).fill(0);
  const labels = new Array(MAX_POINTS).fill('');

  const chart = new Chart(canvas, {
    type: 'line',
    data: {
      labels,
      datasets: [
        {
          label: 'Download (Rx)',
          data: rxData,
          borderColor: '#6366f1',
          backgroundColor: 'rgba(99,102,241,0.08)',
          borderWidth: 2,
          pointRadius: 0,
          fill: true,
          tension: 0.4,
        },
        {
          label: 'Upload (Tx)',
          data: txData,
          borderColor: '#10b981',
          backgroundColor: 'rgba(16,185,129,0.06)',
          borderWidth: 2,
          pointRadius: 0,
          fill: true,
          tension: 0.4,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: true,
      animation: { duration: 300 },
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: ctx => {
              const bps = ctx.parsed.y;
              return ` ${formatBps(bps)}`;
            },
          },
          backgroundColor: '#1e293b',
          borderColor: '#334155',
          borderWidth: 1,
          titleColor: '#94a3b8',
          bodyColor: '#e2e8f0',
        },
      },
      scales: {
        x: { display: false },
        y: {
          display: true,
          grid: { color: 'rgba(51,65,85,0.5)' },
          ticks: {
            color: '#64748b',
            font: { size: 10 },
            callback: v => formatBps(v),
          },
        },
      },
    },
  });

  function formatBps(bps) {
    if (bps >= 1e9) return (bps / 1e9).toFixed(1) + ' Gbps';
    if (bps >= 1e6) return (bps / 1e6).toFixed(1) + ' Mbps';
    if (bps >= 1e3) return (bps / 1e3).toFixed(1) + ' Kbps';
    return bps + ' bps';
  }

  function fetchTraffic() {
    const iface = document.getElementById('iface-select')?.value || 'ether1';
    fetch(`/api/traffic?interface=${iface}`)
      .then(r => r.json())
      .then(d => {
        rxData.push(d.rx || 0);
        txData.push(d.tx || 0);
        labels.push('');
        if (rxData.length > MAX_POINTS) { rxData.shift(); txData.shift(); labels.shift(); }
        chart.update('none');
      })
      .catch(() => {});
  }

  fetchTraffic();
  const interval = setInterval(fetchTraffic, 3000);

  // Change interface
  document.getElementById('iface-select')?.addEventListener('change', () => {
    rxData.fill(0); txData.fill(0);
    chart.update();
    fetchTraffic();
  });
});
