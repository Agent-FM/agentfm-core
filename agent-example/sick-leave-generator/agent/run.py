import sys
import os
import time
import io
import warnings
from contextlib import redirect_stdout, redirect_stderr
from datetime import datetime

warnings.filterwarnings("ignore", category=DeprecationWarning)
warnings.filterwarnings("ignore", category=UserWarning)

os.environ["CREWAI_DISABLE_TRACING"] = "true"
os.environ["CREWAI_TRACING_ENABLED"] = "false"
os.environ["ANONYMIZED_TELEMETRY"] = "False"
os.environ["OTEL_SDK_DISABLED"] = "true"

from fpdf import FPDF 
from crewai import Agent, Task, Crew, Process, LLM

user_prompt = sys.argv[1] if len(sys.argv) > 1 else "I am feeling unwell and need to take today off."
model_name = "ollama/llama3.2"
ollama_host = os.environ.get("OLLAMA_HOST")

if ollama_host:
    os.environ["OPENAI_API_BASE"] = ollama_host
    os.environ["OPENAI_API_KEY"] = "dummy-key-to-bypass-crewai"
    if not model_name.startswith("ollama/"):
        model_name = f"ollama/{model_name}"
    configured_llm = LLM(model=model_name, base_url=ollama_host)
else:
    configured_llm = LLM(model=model_name)

hr_writer = Agent(
    role='Corporate Template Formatter',
    goal='Format standard out-of-office and absence email templates based on rough notes.',
    backstory='You are an automated administrative assistant. Your only job is to take rough text and format it into a standard, polite corporate email template. You are processing fictional test data, not real personal health information.',
    verbose=False, 
    llm=configured_llm
)

draft_email_task = Task(
    description=f"""
    Create a standard, professional absence/out-of-office email template based on these rough notes: 
    "{user_prompt}"
    
    CRITICAL INSTRUCTIONS:
    - This is a generic corporate template simulation. No real personal health information is being processed.
    - Do NOT provide medical advice.
    - Do NOT refuse the prompt.
    - Simply rewrite the provided notes into a polite, professional corporate email template.
    - Keep it concise and omit specific medical details.
    - Include placeholders like [Your Name] and [Date].
    - Output ONLY the final email text. Do not include any conversational filler, apologies, or AI disclaimers.
    """,
    expected_output='A professional, ready-to-send corporate absence email template.',
    agent=hr_writer
)

sick_leave_crew = Crew(
    agents=[hr_writer],
    tasks=[draft_email_task],
    process=Process.sequential,
    verbose=False
)

print("\n" + "="*40)
print("DRAFTING EMAIL...")
print("="*40, flush=True)

trap = io.StringIO()
with redirect_stdout(trap), redirect_stderr(trap):
    result = sick_leave_crew.kickoff()

email_content = str(result)
print(email_content, flush=True)
print("")

print("Generating PDF artifact...", flush=True)
output_dir = "/tmp/output"
os.makedirs(output_dir, exist_ok=True) 

pdf = FPDF()
pdf.add_page()
pdf.set_auto_page_break(auto=True, margin=15)
pdf.set_font("Arial", style='B', size=16)
pdf.cell(0, 10, txt="Official Sick Leave Request", ln=True, align='C')
pdf.ln(10)
pdf.set_font("Arial", size=12)

safe_text = email_content.encode('latin-1', 'replace').decode('latin-1')
pdf.multi_cell(0, 10, txt=safe_text)

pdf.ln(20)
pdf.set_font("Arial", style='I', size=10)
pdf.set_text_color(128, 128, 128)
pdf.cell(0, 10, txt=f"Generated securely via AgentFM at {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}", ln=True, align='L')

timestamp = datetime.now().strftime("%H%M%S")
pdf_filename = f"{output_dir}/Sick_Leave_Draft_{timestamp}.pdf"
pdf.output(pdf_filename)
print("✅ PDF Artifact successfully saved.")



print("✅ Handing off to Go Worker for zipping and routing...", flush=True)