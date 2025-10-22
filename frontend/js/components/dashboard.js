// Dashboard Component

async function loadDashboard() {
    try {
        const data = await API.getDashboard();
        console.log('Dashboard data:', data);
        
        updateDashboardStats(data);
        renderDashboardCharts(data);
    } catch (error) {
        console.error('Error loading dashboard:', error);
        if (error.message !== 'Authentication failed') {
            alert('Failed to load dashboard data');
        }
    }
}

function updateDashboardStats(data) {
    document.getElementById('totalAssets').textContent = Helpers.formatCurrency(data.totalAssets || 0);
    document.getElementById('totalLiabilities').textContent = Helpers.formatCurrency(data.totalLiabilities || 0);
    document.getElementById('netWorth').textContent = Helpers.formatCurrency(data.netWorth || 0);
    document.getElementById('monthIncome').textContent = Helpers.formatCurrency(data.monthIncome || 0);
    document.getElementById('monthExpenses').textContent = Helpers.formatCurrency(data.monthExpenses || 0);
    document.getElementById('monthSavings').textContent = Helpers.formatCurrency(data.monthSavings || 0);
    document.getElementById('monthSavings2').textContent = Helpers.formatCurrency(data.monthSavings || 0);
}

function renderDashboardCharts(data) {
    Charts.renderMonthlyOverview(data);
    Charts.renderBudgetChart(data.budgetVsExpenses || []);
    Charts.renderProgressChart(data.historicalData || [], {
        netWorth: data.netWorth || 0,
        liabilities: data.totalLiabilities || 0,
        savings: data.monthSavings || 0
    });
    Charts.renderAssetDistribution(data);
    renderQuickStats(data);
}

function renderQuickStats(data) {
    const container = document.getElementById('quickStatsContainer');
    if (!container) return;
    
    const totalBudget = (data.budgetVsExpenses || []).reduce((sum, item) => sum + item.budget, 0);
    const budgetRemaining = totalBudget - (data.monthExpenses || 0);
    const savingsRate = data.monthIncome > 0 
        ? Helpers.calculatePercentage(data.monthSavings, data.monthIncome)
        : 0;
    
    let netWorthGrowth = 0;
    if (data.historicalData && data.historicalData.length >= 2) {
        const latest = data.historicalData[data.historicalData.length - 1].netWorth;
        const previous = data.historicalData[data.historicalData.length - 2].netWorth;
        netWorthGrowth = ((latest - previous) / Math.abs(previous) * 100).toFixed(1);
    }
    
    const debtToAsset = data.totalAssets > 0 
        ? Helpers.calculatePercentage(data.totalLiabilities, data.totalAssets)
        : 0;
    
    const stats = [
        {
            label: 'Savings Rate',
            value: savingsRate + '%',
            change: parseFloat(savingsRate) > 20 ? 'Excellent!' : 'Keep going!',
            positive: parseFloat(savingsRate) > 20
        },
        {
            label: 'Budget Remaining',
            value: 'Rs ' + budgetRemaining.toFixed(0),
            change: budgetRemaining > 0 ? 'Within budget' : 'Over budget',
            positive: budgetRemaining > 0
        },
        {
            label: 'Net Worth Growth',
            value: netWorthGrowth + '%',
            change: parseFloat(netWorthGrowth) >= 0 ? 'Growing' : 'Declining',
            positive: parseFloat(netWorthGrowth) >= 0
        },
        {
            label: 'Debt to Asset',
            value: debtToAsset + '%',
            change: 'Ratio',
            positive: data.totalAssets > data.totalLiabilities
        }
    ];
    
    const html = stats.map(stat => `
        <div class="quick-stat-item">
            <div class="quick-stat-label">${stat.label}</div>
            <div class="quick-stat-value">${stat.value}</div>
            <div class="quick-stat-change ${stat.positive ? 'positive' : 'negative'}">
                <span class="material-icons">${stat.positive ? 'trending_up' : 'trending_down'}</span>
                ${stat.change}
            </div>
        </div>
    `).join('');
    
    container.innerHTML = html;
}