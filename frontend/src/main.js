// burnscope frontend
let isRecording = false;
let isComparing = false;

// DOM 元素
const btnRecord = document.getElementById('btn-record');
const btnCompare = document.getElementById('btn-compare');
const portInfo = document.getElementById('port-info');
const recordsDiv = document.getElementById('records');
const matchedSpan = document.getElementById('matched');
const diffSpan = document.getElementById('diff');

// 按钮事件
btnRecord.addEventListener('click', async () => {
    if (!isRecording) {
        // TODO: 调用后端开始录制
        isRecording = true;
        btnRecord.textContent = '停止';
        btnRecord.classList.remove('secondary');
        btnRecord.classList.add('primary');
        portInfo.textContent = '录制中...';
    } else {
        // TODO: 调用后端停止录制
        isRecording = false;
        btnRecord.textContent = '录制';
        btnRecord.classList.remove('primary');
        btnRecord.classList.add('secondary');
        portInfo.textContent = '录制完成';
    }
});

btnCompare.addEventListener('click', async () => {
    if (!isComparing) {
        // TODO: 调用后端开始对比
        isComparing = true;
        btnCompare.textContent = '停止';
        btnCompare.classList.remove('secondary');
        btnCompare.classList.add('primary');
        portInfo.textContent = '虚拟串口: /dev/pts/N';
    } else {
        // TODO: 调用后端停止对比
        isComparing = false;
        btnCompare.textContent = '对比';
        btnCompare.classList.remove('primary');
        btnCompare.classList.add('secondary');
        portInfo.textContent = '就绪';
    }
});

// 添加记录行
function addRecord(index, dir, name, data, match) {
    const row = document.createElement('div');
    row.className = 'record-row';
    
    const dirClass = dir.toLowerCase();
    const matchIcon = match === true ? '✓' : (match === false ? '✗' : '');
    const matchClass = match === true ? 'match' : (match === false ? 'diff' : '');
    
    row.innerHTML = `
        <span class="label">基准:</span>
        <span class="dir ${dirClass}">${dir}</span>
        <span class="name">${name}</span>
        <span class="data">${formatHex(data)}</span>
        <span class="${matchClass}">${matchIcon}</span>
    `;
    
    recordsDiv.appendChild(row);
}

// 格式化十六进制
function formatHex(data) {
    if (!data) return '';
    // 假设 data 是字节数组的字符串表示
    return data;
}

// 更新统计
function updateStats(matched, diff) {
    matchedSpan.textContent = matched;
    diffSpan.textContent = diff;
}
