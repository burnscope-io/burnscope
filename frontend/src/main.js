// burnscope frontend

let runtime;
let state = {
    mode: 'record',
    isRunning: false
};

// DOM Elements
const modeTabs = document.querySelectorAll('.mode-tab');
const actionBtn = document.getElementById('action-btn');
const statusDot = document.getElementById('status-dot');
const statusText = document.getElementById('status-text');
const recordsDiv = document.getElementById('records');
const emptyState = document.getElementById('empty-state');
const proxySingle = document.getElementById('proxy-single');
const proxyDual = document.getElementById('proxy-dual');
const proxyPath = document.getElementById('proxy-path');
const proxyPath1 = document.getElementById('proxy-path-1');
const proxyPath2 = document.getElementById('proxy-path-2');
const baudDisplay = document.getElementById('baud-display');

const recordConfig = document.getElementById('record-config');
const testConfig = document.getElementById('test-config');
const compareConfig = document.getElementById('compare-config');

const devicePort = document.getElementById('device-port');
const baudRate = document.getElementById('baud-rate');
const goldenFile = document.getElementById('golden-file');

const statTx = document.getElementById('stat-tx');
const statRx = document.getElementById('stat-rx');
const statMatch = document.getElementById('stat-match');
const statDiff = document.getElementById('stat-diff');

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
    runtime = window['@wailsapp/runtime'];
    
    // Load serial ports
    await loadPorts();
    
    // Mode tabs
    modeTabs.forEach(tab => {
        tab.addEventListener('click', () => switchMode(tab.dataset.mode));
    });
    
    // Action button
    actionBtn.addEventListener('click', toggleAction);
    
    // Listen for events
    runtime.EventsOn('record', onRecord);
    runtime.EventsOn('compare', onCompare);
    runtime.EventsOn('replay', onReplay);
    runtime.EventsOn('stats', onStats);
    runtime.EventsOn('connected', onConnected);
    runtime.EventsOn('error', onError);
    
    // Refresh ports periodically
    setInterval(loadPorts, 3000);
});

// Load serial ports
async function loadPorts() {
    try {
        const ports = await window.go.main.App.ListSerialPorts();
        devicePort.innerHTML = '<option value="">等待自动连接...</option>';
        ports.forEach(port => {
            const opt = document.createElement('option');
            opt.value = port;
            opt.textContent = port;
            devicePort.appendChild(opt);
        });
    } catch (e) {
        console.error('Failed to load ports:', e);
    }
}

// Switch mode
function switchMode(mode) {
    if (state.isRunning) return;
    
    state.mode = mode;
    
    modeTabs.forEach(tab => {
        tab.classList.toggle('active', tab.dataset.mode === mode);
    });
    
    // Show/hide config sections
    recordConfig.classList.toggle('hidden', mode !== 'record');
    testConfig.classList.toggle('hidden', mode !== 'test');
    compareConfig.classList.toggle('hidden', mode !== 'compare');
    
    // Update button text
    switch (mode) {
        case 'record':
            actionBtn.textContent = '开始录制';
            break;
        case 'test':
            actionBtn.textContent = '开始测试';
            break;
        case 'compare':
            actionBtn.textContent = '开始对比';
            break;
    }
    
    // Reset display
    proxySingle.classList.remove('hidden');
    proxyDual.classList.add('hidden');
    proxyPath.textContent = '-';
    clearRecords();
    updateStatus('idle');
}

// Toggle action
async function toggleAction() {
    if (state.isRunning) {
        await stop();
    } else {
        await start();
    }
}

// Start
async function start() {
    try {
        clearRecords();
        
        switch (state.mode) {
            case 'record':
                await startRecord();
                break;
            case 'test':
                await startTest();
                break;
            case 'compare':
                await startCompare();
                break;
        }
    } catch (e) {
        alert('启动失败: ' + e);
    }
}

// Start record mode
async function startRecord() {
    const port = devicePort.value;
    const baud = parseInt(baudRate.value) || 0;
    
    const proxy = await window.go.main.App.StartRecord(port);
    
    state.isRunning = true;
    actionBtn.textContent = '停止';
    actionBtn.classList.add('stop');
    updateStatus('running');
    
    proxySingle.classList.remove('hidden');
    proxyDual.classList.add('hidden');
    proxyPath.textContent = proxy;
    
    // If device port specified and baud set, connect
    if (port && baud > 0) {
        await window.go.main.App.ConnectDevice(port, baud);
        baudDisplay.textContent = `波特率: ${baud}`;
    } else {
        baudDisplay.textContent = '等待捕获波特率...';
    }
}

// Start test mode
async function startTest() {
    const [proxy1, proxy2] = await window.go.main.App.StartTest();
    
    state.isRunning = true;
    actionBtn.textContent = '停止';
    actionBtn.classList.add('stop');
    updateStatus('running');
    
    proxySingle.classList.add('hidden');
    proxyDual.classList.remove('hidden');
    proxyPath1.textContent = proxy1;
    proxyPath2.textContent = proxy2;
    baudDisplay.textContent = '';
}

// Start compare mode
async function startCompare() {
    const file = goldenFile.value || 'session.golden';
    
    // Try to load session first
    try {
        await window.go.main.App.LoadSession(file);
    } catch (e) {
        alert('加载基准文件失败: ' + e);
        return;
    }
    
    const proxy = await window.go.main.App.StartCompare();
    
    state.isRunning = true;
    actionBtn.textContent = '停止';
    actionBtn.classList.add('stop');
    updateStatus('running');
    
    proxySingle.classList.remove('hidden');
    proxyDual.classList.add('hidden');
    proxyPath.textContent = proxy;
    baudDisplay.textContent = '';
}

// Stop
async function stop() {
    try {
        await window.go.main.App.Stop();
        
        state.isRunning = false;
        actionBtn.classList.remove('stop');
        baudDisplay.textContent = '';
        
        switch (state.mode) {
            case 'record':
                actionBtn.textContent = '开始录制';
                // Prompt to save
                if (confirm('是否保存录制数据？')) {
                    const path = goldenFile.value || 'session.golden';
                    await window.go.main.App.SaveSession(path);
                    alert('已保存: ' + path);
                }
                break;
            case 'test':
                actionBtn.textContent = '开始测试';
                break;
            case 'compare':
                actionBtn.textContent = '开始对比';
                break;
        }
        
        updateStatus('idle');
    } catch (e) {
        console.error('Stop failed:', e);
    }
}

// Update status
function updateStatus(status) {
    statusDot.className = 'status-dot';
    if (status === 'running') {
        statusDot.classList.add('running');
        statusText.textContent = '运行中';
    } else {
        statusText.textContent = '就绪';
    }
}

// Clear records
function clearRecords() {
    recordsDiv.innerHTML = '';
    emptyState.style.display = 'flex';
    recordsDiv.appendChild(emptyState);
    statTx.textContent = '0';
    statRx.textContent = '0';
    statMatch.textContent = '0';
    statDiff.textContent = '0';
}

// On record event
function onRecord(data) {
    emptyState.style.display = 'none';
    
    const row = document.createElement('div');
    row.className = 'record-row';
    
    const dirClass = data.direction.toLowerCase();
    row.innerHTML = `
        <span class="dir ${dirClass}">${data.direction}</span>
        <span class="size">${data.size}B</span>
        <span class="data">${truncateHex(data.data, 48)}</span>
    `;
    
    recordsDiv.appendChild(row);
    recordsDiv.scrollTop = recordsDiv.scrollHeight;
    
    // Update counters
    if (data.direction === 'TX') {
        statTx.textContent = parseInt(statTx.textContent) + 1;
    } else {
        statRx.textContent = parseInt(statRx.textContent) + 1;
    }
}

// On compare event
function onCompare(data) {
    emptyState.style.display = 'none';
    
    // Expected row
    if (data.expected) {
        const expectedRow = document.createElement('div');
        expectedRow.className = 'record-row';
        expectedRow.innerHTML = `
            <span class="dir tx">TX</span>
            <span class="size">${data.expected.data.length / 2}B</span>
            <span class="data">基准: ${truncateHex(data.expected.data, 40)}</span>
        `;
        recordsDiv.appendChild(expectedRow);
    }
    
    // Actual row
    const actualRow = document.createElement('div');
    actualRow.className = 'record-row';
    const matchClass = data.match ? 'match' : 'diff';
    const matchIcon = data.match ? '✓' : '✗';
    actualRow.innerHTML = `
        <span class="dir tx">TX</span>
        <span class="size">${data.actual.data.length / 2}B</span>
        <span class="data">对比: ${truncateHex(data.actual.data, 40)}</span>
        <span class="result ${matchClass}">${matchIcon}</span>
    `;
    recordsDiv.appendChild(actualRow);
    
    const divider = document.createElement('div');
    divider.className = 'divider';
    recordsDiv.appendChild(divider);
    
    recordsDiv.scrollTop = recordsDiv.scrollHeight;
}

// On replay event
function onReplay(data) {
    const row = document.createElement('div');
    row.className = 'record-row';
    row.innerHTML = `
        <span class="dir rx">RX</span>
        <span class="size">${data.data.length / 2}B</span>
        <span class="data" style="color: #238636;">回放: ${truncateHex(data.data, 40)}</span>
    `;
    recordsDiv.appendChild(row);
    recordsDiv.scrollTop = recordsDiv.scrollHeight;
}

// On stats event
function onStats(data) {
    statMatch.textContent = data.matched;
    statDiff.textContent = data.diff;
}

// On connected event
function onConnected(data) {
    baudDisplay.textContent = `已连接: ${data.device} @ ${data.baud}`;
}

// On error event
function onError(msg) {
    alert('错误: ' + msg);
}

// Copy path to clipboard
function copyPath(id) {
    let text;
    if (id) {
        text = document.getElementById(id).textContent;
    } else {
        text = proxyPath.textContent;
    }
    
    if (text && text !== '-') {
        navigator.clipboard.writeText(text).then(() => {
            // Brief visual feedback
            const el = id ? document.getElementById(id) : proxyPath;
            const original = el.style.background;
            el.style.background = '#238636';
            setTimeout(() => el.style.background = original, 200);
        });
    }
}

// Truncate hex string
function truncateHex(hex, maxLen) {
    if (!hex || hex.length <= maxLen) return hex || '';
    return hex.substring(0, maxLen) + '...';
}

// Expose copyPath globally
window.copyPath = copyPath;
