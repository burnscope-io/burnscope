// burnscope frontend

let state = {
    mode: 'record',
    isRunning: false
};

const portSelect = document.getElementById('port-select');
const baudSelect = document.getElementById('baud-select');
const actionBtn = document.getElementById('action-btn');
const statusDot = document.getElementById('status-dot');
const statusText = document.getElementById('status-text');
const recordsDiv = document.getElementById('records');
const emptyState = document.getElementById('empty-state');
const statMatch = document.getElementById('stat-match');
const statDiff = document.getElementById('stat-diff');
const ptyPath = document.getElementById('pty-path');
const modeTabs = document.querySelectorAll('.mode-tab');

document.addEventListener('DOMContentLoaded', async () => {
    await loadPorts();
    
    modeTabs.forEach(tab => {
        tab.addEventListener('click', () => switchMode(tab.dataset.mode));
    });
    
    actionBtn.addEventListener('click', toggleAction);
    
    setInterval(loadPorts, 3000);
});

async function loadPorts() {
    try {
        const ports = await window.go.main.App.ListSerialPorts();
        portSelect.innerHTML = '';
        
        if (ports.length === 0) {
            portSelect.innerHTML = '<option value="">未检测到串口</option>';
            return;
        }
        
        ports.forEach(port => {
            const opt = document.createElement('option');
            opt.value = port;
            opt.textContent = port;
            portSelect.appendChild(opt);
        });
    } catch (e) {
        portSelect.innerHTML = '<option value="">加载失败</option>';
    }
}

function switchMode(mode) {
    if (state.isRunning) return;
    
    state.mode = mode;
    modeTabs.forEach(tab => {
        tab.classList.toggle('active', tab.dataset.mode === mode);
    });
    
    actionBtn.textContent = mode === 'record' ? '开始录制' : '开始对比';
    ptyPath.textContent = '';
    updateStatus('idle');
}

async function toggleAction() {
    if (state.isRunning) {
        await stop();
    } else {
        await start();
    }
}

async function start() {
    const port = portSelect.value;
    const baud = parseInt(baudSelect.value);
    
    if (state.mode === 'record') {
        if (!port) { alert('请选择串口'); return; }
        
        try {
            await window.go.main.App.StartRecording(port, baud);
            state.isRunning = true;
            actionBtn.textContent = '停止录制';
            actionBtn.classList.add('stop');
            updateStatus('recording');
            clearRecords();
        } catch (e) {
            alert('启动录制失败: ' + e);
        }
    } else {
        try {
            const pty = await window.go.main.App.StartCompare();
            state.isRunning = true;
            actionBtn.textContent = '停止对比';
            actionBtn.classList.add('stop');
            updateStatus('comparing');
            ptyPath.textContent = '虚拟串口: ' + pty;
            clearRecords();
        } catch (e) {
            alert('启动对比失败: ' + e);
        }
    }
}

async function stop() {
    try {
        if (state.mode === 'record') {
            await window.go.main.App.StopRecording();
        } else {
            await window.go.main.App.StopCompare();
        }
        
        state.isRunning = false;
        actionBtn.textContent = state.mode === 'record' ? '开始录制' : '开始对比';
        actionBtn.classList.remove('stop');
        updateStatus('idle');
    } catch (e) {
        console.error('Stop failed:', e);
    }
}

function updateStatus(status) {
    statusDot.className = 'status-dot';
    switch (status) {
        case 'recording':
            statusDot.classList.add('recording');
            statusText.textContent = '录制中';
            break;
        case 'comparing':
            statusDot.classList.add('comparing');
            statusText.textContent = '对比中';
            break;
        default:
            statusText.textContent = '就绪';
    }
}

function clearRecords() {
    recordsDiv.innerHTML = '';
    emptyState.style.display = 'flex';
    recordsDiv.appendChild(emptyState);
    statMatch.textContent = '0';
    statDiff.textContent = '0';
}

function addRecord(direction, data, size) {
    emptyState.style.display = 'none';
    
    const row = document.createElement('div');
    row.className = 'record-row';
    row.innerHTML = `
        <span class="dir ${direction.toLowerCase()}">${direction}</span>
        <span class="size">${size}B</span>
        <span class="data">${formatHex(data)}</span>
    `;
    recordsDiv.appendChild(row);
    recordsDiv.scrollTop = recordsDiv.scrollHeight;
}

function addCompareLine(expected, actual, match) {
    emptyState.style.display = 'none';
    
    // 基准行
    if (expected) {
        const baseline = document.createElement('div');
        baseline.className = 'record-row baseline';
        baseline.innerHTML = `
            <span class="label">基准</span>
            <span class="dir ${expected.direction.toLowerCase()}">${expected.direction}</span>
            <span class="size">${expected.data.length / 2}B</span>
            <span class="data">${formatHex(expected.data)}</span>
        `;
        recordsDiv.appendChild(baseline);
    }
    
    // 对比行
    const compare = document.createElement('div');
    compare.className = 'record-row compare';
    const resultIcon = match ? '✓' : '✗';
    const resultClass = match ? 'match' : 'diff';
    compare.innerHTML = `
        <span class="label">对比</span>
        <span class="dir ${actual.direction.toLowerCase()}">${actual.direction}</span>
        <span class="size">${actual.data.length / 2}B</span>
        <span class="data">${formatHex(actual.data)}</span>
        <span class="result ${resultClass}">${resultIcon}</span>
    `;
    recordsDiv.appendChild(compare);
    
    const divider = document.createElement('div');
    divider.className = 'divider';
    recordsDiv.appendChild(divider);
    
    recordsDiv.scrollTop = recordsDiv.scrollHeight;
}

function formatHex(data) {
    if (!data) return '';
    return data.length > 64 ? data.substring(0, 64) + '...' : data;
}

function updateStats(matched, diff) {
    statMatch.textContent = matched;
    statDiff.textContent = diff;
}

window.burnscope = { addRecord, addCompareLine, updateStats };
