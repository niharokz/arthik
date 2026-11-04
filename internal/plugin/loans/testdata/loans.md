# arthik loans plugin — your EMI loans. Arthik only reads this file; edit it when the lender sends a new schedule or statement.
# rate is % a year. Amounts are rupees. schedule_text: the lender's table as pasted; EMIs after its last row are estimated from the rate.
# account = the Arthik loan account (principal), pay_from = where the EMI is debited, interest_category = where interest is spent.
updated: '2026-11-04 12:00:00'

loans:
  - id: lic-hl
    name: LIC Home Loan
    lender: LIC Housing Finance
    loan_number: '9002050007113'
    area_office: Bhubaneswar
    account: lic-homeloan
    pay_from: sbi-savings
    interest_category: home-loan-interest
    sanction_date: '2025-07-03'
    sanction_amount: '4950000.00'
    first_disbursement: '2025-07-23'
    disbursed: '3400000.00'
    rate: '7.85'
    emi: '42054.00'
    emi_day: 5
    end_date: '2035-07-05'
    last_emi: '42025.89'
    fy_start_month: 4
    statement:
      as_of: '2026-11-04'
      principal_outstanding: '3186836.69'
      unpaid_installments: 105
      fy_principal_paid: '141474.24'
      fy_interest_paid: '132666.76'
    # Each year: paste the lender's new rows at the end of schedule_text, exactly as the
    # lender shows them (header lines are fine; rows are matched by due date).
    schedule_text: |
      Due Date	Interest Rate	EMI (Prin.)	EMI (Int.)	Total EMI	Schedule OS
      05-NOV-26	7.85%%	₹ 21,206.77	₹ 20,847.23	₹ 42,054.00	₹ 3,165,629.92
      05-DEC-26	7.85%%	₹ 21,345.51	₹ 20,708.49	₹ 42,054.00	₹ 3,144,284.41
      05-JAN-27	7.85%%	₹ 21,485.14	₹ 20,568.86	₹ 42,054.00	₹ 3,122,799.27
      05-FEB-27	7.85%%	₹ 21,625.68	₹ 20,428.32	₹ 42,054.00	₹ 3,101,173.59
      05-MAR-27	7.85%%	₹ 21,767.16	₹ 20,286.84	₹ 42,054.00	₹ 3,079,406.43
      05-APR-27	7.85%%	₹ 21,909.55	₹ 20,144.45	₹ 42,054.00	₹ 3,057,496.88
      05-MAY-27	7.85%%	₹ 22,052.87	₹ 20,001.13	₹ 42,054.00	₹ 3,035,444.01
      05-JUN-27	7.85%%	₹ 22,197.14	₹ 19,856.86	₹ 42,054.00	₹ 3,013,246.87
      05-JUL-27	7.85%%	₹ 22,342.35	₹ 19,711.65	₹ 42,054.00	₹ 2,990,904.52
      05-AUG-27	7.85%%	₹ 22,488.50	₹ 19,565.50	₹ 42,054.00	₹ 2,968,416.02
      05-SEP-27	7.85%%	₹ 22,635.61	₹ 19,418.39	₹ 42,054.00	₹ 2,945,780.41
      05-OCT-27	7.85%%	₹ 22,783.68	₹ 19,270.32	₹ 42,054.00	₹ 2,922,996.73
