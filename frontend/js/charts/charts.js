// Charts Module - All chart rendering logic

const Charts = {
    // Render monthly overview doughnut chart
    renderMonthlyOverview(data) {
        const ctx = document.getElementById('monthlyOverviewChart');
        if (!ctx) return;
        
        destroyChart('monthlyOverview');
        
        const income = data.monthIncome || 0;
        const expenses = data.monthExpenses || 0;
        const savings = data.monthSavings || 0;
        
        if (income === 0 && expenses === 0) {
            ctx.parentElement.innerHTML = '<div class="no-data-message"><span class="material-icons">insert_chart</span><p>Add transactions to see your monthly overview</p></div>';
            return;
        }
        
        const chart = new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: ['Income', 'Expenses', 'Savings'],
                datasets: [{
                    data: [income, expenses, Math.max(0, savings)],
                    backgroundColor: [
                        'rgba(76, 175, 80, 0.8)',
                        'rgba(244, 67, 54, 0.8)',
                        'rgba(255, 152, 0, 0.8)'
                    ],
                    borderColor: [
                        'rgba(76, 175, 80, 1)',
                        'rgba(244, 67, 54, 1)',
                        'rgba(255, 152, 0, 1)'
                    ],
                    borderWidth: 2
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                plugins: {
                    legend: { display: false },
                    tooltip: {
                        callbacks: {
                            label: function(context) {
                                const label = context.label || '';
                                const value = context.parsed || 0;
                                const total = context.dataset.data.reduce((a, b) => a + b, 0);
                                const percentage = ((value / total) * 100).toFixed(1);
                                return `${label}: Rs ${value.toFixed(2)} (${percentage}%)`;
                            }
                        }
                    }
                }
            }
        });
        
        setChart('monthlyOverview', chart);
    },
    
    // Render budget vs expenses bar chart
    renderBudgetChart(budgetData) {
        const ctx = document.getElementById('budgetChart');
        if (!ctx) return;
        
        destroyChart('budget');
        
        if (!budgetData || budgetData.length === 0) {
            ctx.parentElement.innerHTML = '<div class="no-data-message"><span class="material-icons">bar_chart</span><p>Set budgets in expense accounts to see comparison</p></div>';
            return;
        }
        
        const categories = budgetData.map(item => item.category);
        const budgets = budgetData.map(item => item.budget);
        const actuals = budgetData.map(item => item.actual);
        const isMobile = Helpers.isMobile();
        
        const chart = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: categories,
                datasets: [{
                    label: 'Budget',
                    data: budgets,
                    backgroundColor: 'rgba(66, 165, 245, 0.6)',
                    borderColor: 'rgba(66, 165, 245, 1)',
                    borderWidth: 2,
                    borderRadius: 8
                }, {
                    label: 'Actual',
                    data: actuals,
                    backgroundColor: 'rgba(255, 152, 0, 0.6)',
                    borderColor: 'rgba(255, 152, 0, 1)',
                    borderWidth: 2,
                    borderRadius: 8
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                scales: {
                    y: {
                        beginAtZero: true,
                        ticks: {
                            callback: (value) => 'Rs ' + value.toLocaleString(),
                            font: { size: isMobile ? 9 : 11 }
                        },
                        grid: { color: 'rgba(0, 0, 0, 0.05)' }
                    },
                    x: {
                        grid: { display: false },
                        ticks: {
                            font: { size: isMobile ? 9 : 11 },
                            maxRotation: isMobile ? 45 : 0,
                            minRotation: isMobile ? 45 : 0
                        }
                    }
                },
                plugins: {
                    legend: {
                        display: true,
                        position: 'top',
                        labels: {
                            usePointStyle: true,
                            padding: isMobile ? 10 : 15,
                            font: { size: isMobile ? 10 : 12 }
                        }
                    },
                    tooltip: {
                        callbacks: {
                            label: (context) => `${context.dataset.label}: Rs ${context.parsed.y.toFixed(2)}`
                        }
                    }
                }
            }
        });
        
        setChart('budget', chart);
    },
    
    // Render progress line chart
    renderProgressChart(historicalData, currentData) {
        const ctx = document.getElementById('progressChart');
        if (!ctx) return;
        
        destroyChart('progress');
        
        let dataToDisplay = [];
        
        if (!historicalData || historicalData.length === 0) {
            const today = new Date();
            dataToDisplay = [{
                date: today.toISOString().split('T')[0],
                netWorth: currentData.netWorth,
                liabilities: currentData.liabilities,
                savings: currentData.savings
            }];
        } else {
            dataToDisplay = historicalData.slice(-6);
        }
        
        const labels = dataToDisplay.map(item => {
            const date = new Date(item.date);
            return date.toLocaleDateString('en-IN', { month: 'short', year: 'numeric' });
        });
        
        const netWorthData = dataToDisplay.map(item => item.netWorth);
        const liabilitiesData = dataToDisplay.map(item => item.liabilities);
        const savingsData = dataToDisplay.map(item => item.savings);
        const isMobile = Helpers.isMobile();
        
        const chart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Net Worth',
                    data: netWorthData,
                    borderColor: 'rgba(76, 175, 80, 1)',
                    backgroundColor: 'rgba(76, 175, 80, 0.1)',
                    borderWidth: isMobile ? 2 : 3,
                    tension: 0.4,
                    fill: true,
                    pointRadius: isMobile ? 3 : 5,
                    pointHoverRadius: isMobile ? 5 : 7
                }, {
                    label: 'Liabilities',
                    data: liabilitiesData,
                    borderColor: 'rgba(244, 67, 54, 1)',
                    backgroundColor: 'rgba(244, 67, 54, 0.1)',
                    borderWidth: 2,
                    tension: 0.4,
                    fill: false,
                    pointRadius: isMobile ? 2 : 4,
                    pointHoverRadius: isMobile ? 4 : 6
                }, {
                    label: 'Monthly Savings',
                    data: savingsData,
                    borderColor: 'rgba(255, 152, 0, 1)',
                    backgroundColor: 'rgba(255, 152, 0, 0.1)',
                    borderWidth: 2,
                    tension: 0.4,
                    fill: false,
                    pointRadius: isMobile ? 2 : 4,
                    pointHoverRadius: isMobile ? 4 : 6
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: { intersect: false, mode: 'index' },
                scales: {
                    y: {
                        beginAtZero: true,
                        ticks: {
                            callback: (value) => 'Rs ' + value.toLocaleString(),
                            font: { size: isMobile ? 9 : 11 }
                        },
                        grid: { color: 'rgba(0, 0, 0, 0.05)' }
                    },
                    x: {
                        grid: { display: false },
                        ticks: {
                            font: { size: isMobile ? 9 : 11 },
                            maxRotation: isMobile ? 45 : 0,
                            minRotation: isMobile ? 45 : 0
                        }
                    }
                },
                plugins: {
                    legend: {
                        display: true,
                        position: 'top',
                        labels: {
                            usePointStyle: true,
                            padding: isMobile ? 10 : 20,
                            font: { size: isMobile ? 10 : 12 }
                        }
                    },
                    tooltip: {
                        callbacks: {
                            label: (context) => `${context.dataset.label}: Rs ${context.parsed.y.toFixed(2)}`
                        }
                    }
                }
            }
        });
        
        setChart('progress', chart);
    },
    
    // Render asset distribution pie chart
    renderAssetDistribution(data) {
        const ctx = document.getElementById('assetDistributionChart');
        if (!ctx) return;
        
        destroyChart('assetDistribution');
        
        const accounts = getAccounts();
        const assetAccounts = accounts.filter(acc => 
            acc.category === 'Assets' && acc.currentBalance > 0
        );
        
        if (assetAccounts.length === 0) {
            ctx.parentElement.innerHTML = '<div class="no-data-message"><span class="material-icons">donut_large</span><p>Add assets to see distribution</p></div>';
            return;
        }
        
        const labels = assetAccounts.map(acc => acc.name);
        const values = assetAccounts.map(acc => acc.currentBalance);
        const colors = [
            'rgba(66, 165, 245, 0.8)',
            'rgba(76, 175, 80, 0.8)',
            'rgba(255, 152, 0, 0.8)',
            'rgba(156, 39, 176, 0.8)',
            'rgba(255, 193, 7, 0.8)',
            'rgba(0, 188, 212, 0.8)'
        ];
        const isMobile = Helpers.isMobile();
        
        const chart = new Chart(ctx, {
            type: 'pie',
            data: {
                labels: labels,
                datasets: [{
                    data: values,
                    backgroundColor: colors.slice(0, labels.length),
                    borderColor: colors.slice(0, labels.length).map(c => c.replace('0.8', '1')),
                    borderWidth: 2
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                plugins: {
                    legend: {
                        display: true,
                        position: isMobile ? 'bottom' : 'right',
                        labels: {
                            padding: isMobile ? 10 : 15,
                            usePointStyle: true,
                            font: { size: isMobile ? 10 : 12 }
                        }
                    },
                    tooltip: {
                        callbacks: {
                            label: function(context) {
                                const label = context.label || '';
                                const value = context.parsed || 0;
                                const total = context.dataset.data.reduce((a, b) => a + b, 0);
                                const percentage = ((value / total) * 100).toFixed(1);
                                return `${label}: Rs ${value.toFixed(2)} (${percentage}%)`;
                            }
                        }
                    }
                }
            }
        });
        
        setChart('assetDistribution', chart);
    }
};