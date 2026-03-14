// burnscope frontend

let runtime;
let state = {
    isRunning: false,
    lowerType: 'virtual',
    upperPort: '',
    lowerPort: ''
};

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
    runtime = window['@wailsapp/runtime'];
    
    // 监听事件
    runtime.EventsOn('ports_ready', onPortsReady);
    runtime.EventsOn('data', onData);
    runtime.EventsOn('compare', onCompare);
    runtime.EventsOn('stats', onStats);
    runtime.EventsOn('error', onError);
    
    // 获取端口
    try {
        state.upperPort = await window.go.main.App.GetUpperPort();
        state.lowerPort = await window.go.main.App.GetLowerPort();
        updatePorts();
    } catch (e) {
        console.error('Get ports failed:', e);
    }
    
    // 加载物理串口列表
    await loadPhysicalPorts();
    setInterval(loadPhysicalPorts, 3000);
});

// 更新端口显示
function updatePorts() {
    document.getElementById('upper-port').textContent = state.upperPort || '初始化中...';
    document.getElementById('lower-port').textContent = state.lowerPort || '初始化中...';
}

// 加载物理串口
async function loadPhysicalPorts() {
    try {
        const ports = await window.go.main.App.ListSerialPorts();
        const select = document.getElementById('physical-port');
        select.innerHTML = '';
        ports.forEach(port => {
            const opt = document.createElement('option');
            opt.value = port;
            opt.textContent = port;
            select.appendChild(opt);
        });
    } catch (e) {
        console.error('Load ports failed:', e);
    }
}

// 下位类型变化
function onLowerTypeChange() {
    state.lowerType = document.getElementById('lower-type').value;
    
    document.getElementById('virtual-config').style.display = state.lowerType === 'virtual' ? 'block' : 'none';
    document.getElementById('physical-config').style.display = state.lowerType === 'physical' ? 'block' : 'none';
    document.getElementById('compare-config').style.display = state.lowerType === 'compare' ? 'block' : 'none';
    
    // 更新按钮文本
    const btn = document.getElementById('action-btn');
    switch (state.lowerType) {
        case 'virtual':
            btn.textContent = '开始模拟';
            break;
        case 'physical':
            btn.textContent = '开始录制';
            break;
        case 'compare':
            btn.textContent = '开始对比';
            break;
    }
}

// 开始/停止
async function toggleAction() {
    if (state.isRunning) {
        await stop();
    } else {
        await start();
    }
}

// 开始
async function start() {
    const btn = document.getElementById('action-btn');
    
    try {
        switch (state.lowerType) {
            case 'virtual':
                await window.go.main.App.StartSimulate();
                break;
            case 'physical':
                const port = document.getElementById('physical-port').value;
                const baud = parseInt(document.getElementById('baud-rate').value);
                await window.go.main.App.StartRecord(port, baud);
                break;
            case 'compare':
                await window.go.main.App.StartCompare();
                break;
        }
        
        state.isRunning = true;
        btn.textContent = '停止';
        btn.classList.add('stop');
        document.getElementById('status-dot').classList.add('running');
        document.getElementById('status-text').textContent = '运行中';
        clearRecords();
        
    } catch (e) {
        alert('启动失败: ' + e);
    }
}

// 停止
async function stop() {
    try {
        await window.go.main.App.Stop();
        
        state.isRunning = false;
        const btn = document.getElementById('action-btn');
        btn.classList.remove('stop');
        onLowerTypeChange(); // 恢复按钮文本
        
        document.getElementById('status-dot').classList.remove('running');
        document.getElementById('status-text').textContent = '就绪';
        
    } catch (e) {
        console.error('Stop failed:', e);
    }
}

// 保存录制
async function saveSession() {
    const file = document.getElementById('golden-file').value || 'session.golden';
    try {
        await window.go.main.App.SaveSession(file);
        alert('已保存: ' + file);
    } catch (e) {
        alert('保存失败: ' + e);
    }
}

// 加载基准
async function loadGolden() {
    const file = document.getElementById('golden-file').value || 'session.golden';
    try {
        const count = await window.go.main.App.LoadSession(file);
        document.getElementById('golden-info').textContent = `已加载: ${count} 条记录`;
    } catch (e) {
        alert('加载失败: ' + e);
    }
}

// 清空记录
function clearRecords() {
    const records = document.getElementById('records');
    records.innerHTML = '<div class="empty-state" id="empty-state"><p>等待数据...</p></div>';
    document.getElementById('stat-tx').textContent = '0';
    document.getElementById('stat-rx').textContent = '0';
    document.getElementById('stat-match').textContent = '0';
    document.getElementById('stat-diff').textContent = '0';
}

// 端口就绪
function onPortsReady(data) {
    state.upperPort = data.upper;
    state.lowerPort = data.lower;
    updatePorts();
}

// 数据事件
function onData(data) {
    const records = document.getElementById('records');
    document.getElementById('empty-state').style.display = 'none';
    
    const row = document.createElement('div');
    row.className = 'record-row';
    if (data.replay) row.classList.add('replay');
    
    const dirClass = data.direction.toLowerCase();
    row.innerHTML = `
        <span class="dir ${dirClass}">${data.direction}</span>
        <span class="size">${data.size}B</span>
        <span class="data">${truncateHex(data.data, 48)}${data.replay ? ' (回放)' : ''}</span>
    `;
    
    records.appendChild(row);
    records.scrollTop = records.scrollHeight;
    
    // 更新统计
    if (data.direction === 'TX') {
        const el = document.getElementById('stat-tx');
        el.textContent = parseInt(el.textContent) + 1;
    } else {
        const el = document.getElementById('stat-rx');
        el.textContent = parseInt(el.textContent) + 1;
    }
}

// 对比事件
function onCompare(data) {
    const records = document.getElementById('records');
    document.getElementById('empty-state').style.display = 'none';
    
    // 基准行
    if (data.expected) {
        const expectedRow = document.createElement('div');
        expectedRow.className = 'record-row';
        expectedRow.innerHTML = `
            <span class="dir tx">TX</span>
            <span class="size">${data.expected.data.length / 2}B</span>
            <span class="data">基准: ${truncateHex(data.expected.data, 40)}</span>
        `;
        records.appendChild(expectedRow);
    }
    
    // 对比行
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
    records.appendChild(actualRow);
    
    records.scrollTop = records.scrollHeight;
}

// 统计事件
function onStats(data) {
    document.getElementById('stat-match').textContent = data.matched;
    document.getElementById('stat-diff').textContent = data.diff;
}

// 错误事件
function onError(msg) {
    alert('错误: ' + msg);
}

// 复制端口
function copyPort(type) {
    const port = type === 'upper' ? state.upperPort : state.lowerPort;
    if (port) {
        navigator.clipboard.writeText(port).then(() => {
            const el = document.getElementById(type + '-port');
            const original = el.style.color;
            el.style.color = '#238636';
            setTimeout(() => el.style.color = original, 200);
        });
    }
}

// 截断 hex
function truncateHex(hex, maxLen) {
    if (!hex || hex.length <= maxLen) return hex || '';
    return hex.substring(0, maxLen) + '...';
}

// 暴露全局函数
window.onLowerTypeChange = onLowerTypeChange;
window.toggleAction = toggleAction;
window.saveSession = saveSession;
window.loadGolden = loadGolden;
window.copyPort = copyPort;